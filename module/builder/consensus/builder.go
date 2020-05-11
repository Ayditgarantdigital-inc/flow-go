// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// Builder is the builder for consensus block payloads. Upon providing a payload
// hash, it also memorizes which entities were included into the payload.
type Builder struct {
	metrics  module.MempoolMetrics
	db       *badger.DB
	seals    storage.Seals
	headers  storage.Headers
	payloads storage.Payloads
	blocks   storage.Blocks
	guarPool mempool.Guarantees
	sealPool mempool.Seals
	cfg      Config
}

// NewBuilder creates a new block builder.
func NewBuilder(metrics module.MempoolMetrics, db *badger.DB, headers storage.Headers, seals storage.Seals, payloads storage.Payloads, blocks storage.Blocks, guarPool mempool.Guarantees, sealPool mempool.Seals, options ...func(*Config)) *Builder {

	// initialize default config
	cfg := Config{
		minInterval: 500 * time.Millisecond,
		maxInterval: 10 * time.Second,
		expiry:      flow.DefaultTransactionExpiry,
	}

	// apply option parameters
	for _, option := range options {
		option(&cfg)
	}

	b := &Builder{
		metrics:  metrics,
		db:       db,
		headers:  headers,
		seals:    seals,
		payloads: payloads,
		blocks:   blocks,
		guarPool: guarPool,
		sealPool: sealPool,
		cfg:      cfg,
	}
	return b
}

// BuildOn creates a new block header build on the provided parent, using the given view and applying the
// custom setter function to allow the caller to make changes to the header before storing it.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {

	// STEP ONE: Create a lookup of all previously used guarantees on the part
	// of the chain that we are building on. We do this separately for pending
	// and finalized ancestors, so we can differentiate what to do about it.

	var finalized uint64
	err := b.db.View(operation.RetrieveFinalizedHeight(&finalized))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve finalized height: %w", err)
	}
	var finalID flow.Identifier
	err = b.db.View(operation.LookupBlockHeight(finalized, &finalID))
	if err != nil {
		return nil, fmt.Errorf("could not lookup finalized block: %w", err)
	}

	ancestorID := parentID
	pendingLookup := make(map[flow.Identifier]struct{})
	for ancestorID != finalID {
		ancestor, err := b.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}
		if ancestor.Height <= finalized {
			return nil, fmt.Errorf("should always build on last finalized block")
		}
		payload, err := b.payloads.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
		}
		for _, guarantee := range payload.Guarantees {
			pendingLookup[guarantee.ID()] = struct{}{}
		}
		ancestorID = ancestor.ParentID
	}

	// we look back only as far as the expiry limit for the current height we
	// are building for; any guarantee with a reference block before that can
	// not be included anymore anyway
	parent, err := b.headers.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve parent: %w", err)
	}
	height := parent.Height + 1
	limit := height - uint64(b.cfg.expiry)
	if limit > height { // overflow check
		limit = 0
	}

	ancestorID = finalID
	finalLookup := make(map[flow.Identifier]struct{})
	for {
		ancestor, err := b.headers.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
		}
		payload, err := b.payloads.ByBlockID(ancestorID)
		if err != nil {
			return nil, fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
		}
		for _, guarantee := range payload.Guarantees {
			finalLookup[guarantee.ID()] = struct{}{}
		}
		if ancestor.Height <= limit {
			break
		}
		ancestorID = ancestor.ParentID
	}

	// STEP TWO: Go through the guarantees in our memory pool.
	// 1) If it was already included on the finalized part of the chain, remove
	// it from the memory pool and skip.
	// 2) If the reference block has an expired height, also remove it from the
	// memory pool and skip.
	// 3) If it was already included on the pending part of the chain, skip, but
	// keep in memory pool for now.
	// 4) Otherwise, this guarantee can be included in the payload.

	var guarantees []*flow.CollectionGuarantee
	for _, guarantee := range b.guarPool.All() {
		collID := guarantee.ID()
		_, duplicated := finalLookup[collID]
		if duplicated {
			_ = b.guarPool.Rem(collID)
			continue
		}
		ref, err := b.headers.ByBlockID(guarantee.ReferenceBlockID)
		if errors.Is(err, storage.ErrNotFound) {
			_ = b.guarPool.Rem(collID)
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not get reference block: %w", err)
		}
		if ref.Height < limit {
			_ = b.guarPool.Rem(collID)
			continue
		}
		_, duplicated = pendingLookup[collID]
		if duplicated {
			continue
		}
		guarantees = append(guarantees, guarantee)
	}

	b.metrics.MempoolEntries(metrics.ResourceGuarantee, b.guarPool.Size())

	// STEP FOUR: Get the block seal from the parent and see how far we can
	// extend the chain of sealed blocks with the seals in the memory pool.

	lastSeal, err := b.seals.ByBlockID(parentID)
	if err != nil {
		return nil, fmt.Errorf("could not get last seal: %w", err)
	}
	lastSealed, err := b.headers.ByBlockID(lastSeal.BlockID)
	if err != nil {
		return nil, fmt.Errorf("could not get last sealed: %w", err)
	}

	// we map each seal to the parent of its sealed block; that way, we can
	// retrieve a valid seal that will *follow* each block's own seal
	byParent := make(map[flow.Identifier]*flow.Seal)
	for _, seal := range b.sealPool.All() {
		sealed, err := b.headers.ByBlockID(seal.BlockID)
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not retrieve sealed header (%x): %w", seal.BlockID, err)
		}
		if sealed.Height < lastSealed.Height {
			_ = b.sealPool.Rem(seal.ID())
			continue
		}
		byParent[sealed.ParentID] = seal
	}

	b.metrics.MempoolEntries(metrics.ResourceSeal, b.sealPool.Size())

	// starting at the paren't seal, we try to find a seal to extend the current
	// last sealed block; if we do, we keep going until we don't
	// we also execute a sanity check on whether the execution state of the next
	// seal propely connects to the previous seal
	var seals []*flow.Seal
	for len(byParent) > 0 {
		seal, found := byParent[lastSeal.BlockID]
		if !found {
			break
		}
		if !bytes.Equal(seal.InitialState, lastSeal.FinalState) {
			return nil, fmt.Errorf("seal execution states do not connect")
		}
		delete(byParent, lastSeal.BlockID)
		seals = append(seals, seal)
		lastSeal = seal
	}

	// STEP FOUR: We now have guarantees and seals we can validly include
	// in the payload built on top of the given parent. Now we need to build
	// and store the block header, as well as index the payload contents.

	// build the payload so we can get the hash
	payload := &flow.Payload{
		Identities: nil,
		Guarantees: guarantees,
		Seals:      seals,
	}

	// calculate the timestamp and cutoffs
	timestamp := time.Now().UTC()
	from := parent.Timestamp.Add(b.cfg.minInterval)
	to := parent.Timestamp.Add(b.cfg.maxInterval)

	// adjust timestamp if outside of cutoffs
	if timestamp.Before(from) {
		timestamp = from
	}
	if timestamp.After(to) {
		timestamp = to
	}

	// construct default block on top of the provided parent
	header := &flow.Header{
		ChainID:     parent.ChainID,
		ParentID:    parentID,
		Height:      height,
		Timestamp:   timestamp,
		PayloadHash: payload.Hash(),

		// the following fields should be set by the custom function as needed
		// NOTE: we could abstract all of this away into an interface{} field,
		// but that would be over the top as we will probably always use hotstuff
		View:           0,
		ParentVoterIDs: nil,
		ParentVoterSig: nil,
		ProposerID:     flow.ZeroID,
		ProposerSig:    nil,
	}

	// apply the custom fields setter of the consensus algorithm
	err = setter(header)
	if err != nil {
		return nil, fmt.Errorf("could not apply setter: %w", err)
	}

	// insert the proposal into the database
	proposal := &flow.Block{
		Header:  header,
		Payload: payload,
	}
	err = b.blocks.Store(proposal)
	if err != nil {
		return nil, fmt.Errorf("could ot store proposal: %w", err)
	}

	// update protocol state index for the seal and initialize children index
	blockID := proposal.ID()
	err = operation.RetryOnConflict(b.db.Update, func(tx *badger.Txn) error {
		err = operation.IndexBlockSeal(blockID, lastSeal.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not index proposal seal: %w", err)
		}
		err = operation.InsertBlockChildren(blockID, nil)(tx)
		if err != nil {
			return fmt.Errorf("could not insert empty block children: %w", err)
		}
		return nil
	})

	return header, err
}

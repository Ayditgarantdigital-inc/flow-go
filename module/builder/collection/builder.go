package collection

import (
	"fmt"
	"math"
	"time"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Builder is the builder for collection block payloads. Upon providing a
// payload hash, it also memorizes the payload contents.
//
// NOTE: Builder is NOT safe for use with multiple goroutines. Since the
// HotStuff event loop is the only consumer of this interface and is single
// threaded, this is OK.
type Builder struct {
	db                *badger.DB
	mainHeaders       storage.Headers
	clusterHeaders    storage.Headers
	payloads          storage.ClusterPayloads
	transactions      mempool.Transactions
	maxCollectionSize uint
	expiryBuffer      uint
}

type Opt func(*Builder)

func WithMaxCollectionSize(size uint) Opt {
	return func(b *Builder) {
		b.maxCollectionSize = size
	}
}

func WithExpiryBuffer(buf uint) Opt {
	return func(b *Builder) {
		b.expiryBuffer = buf
	}
}

func NewBuilder(db *badger.DB, headers storage.Headers, payloads storage.ClusterPayloads, transactions mempool.Transactions, opts ...Opt) *Builder {

	b := Builder{
		db:                db,
		mainHeaders:       headers,
		clusterHeaders:    headers,
		payloads:          payloads,
		transactions:      transactions,
		maxCollectionSize: 100,
		expiryBuffer:      15,
	}

	for _, apply := range opts {
		apply(&b)
	}
	return &b
}

// BuildOn creates a new block built on the given parent. It produces a payload
// that is valid with respect to the un-finalized chain it extends.
func (b *Builder) BuildOn(parentID flow.Identifier, setter func(*flow.Header) error) (*flow.Header, error) {
	var proposal cluster.Block

	// first we construct a proposal in-memory, ensuring it is a valid extension
	// of chain state -- this can be done in a read-only transaction
	err := b.db.View(func(tx *badger.Txn) error {

		// STEP ONE: Load some things we need to do our work.

		var parent flow.Header
		err := operation.RetrieveHeader(parentID, &parent)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve parent: %w", err)
		}

		// retrieve the height and ID of the latest finalized block ON THE MAIN CHAIN
		// this is used as the reference point for transaction expiry
		var refChainFinalizedHeight uint64
		err = operation.RetrieveFinalizedHeight(&refChainFinalizedHeight)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve main finalized height: %w", err)
		}
		var refChainFinalizedID flow.Identifier
		err = operation.LookupBlockHeight(refChainFinalizedHeight, &refChainFinalizedID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve main finalized ID: %w", err)
		}

		// retrieve the finalized boundary ON THE CLUSTER CHAIN
		var clusterFinal flow.Header
		err = procedure.RetrieveLatestFinalizedClusterHeader(parent.ChainID, &clusterFinal)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve cluster final: %w", err)
		}

		// STEP TWO: create a lookup of all previously used transactions on the
		// part of the chain we care about. We do this separately for
		// un-finalized and finalized sections of the chain to decide whether to
		// remove conflicting transactions from the mempool.

		// look up previously included transactions in UN-FINALIZED ancestors
		ancestorID := parentID
		unfinalizedLookup := make(map[flow.Identifier]struct{})
		for ancestorID != clusterFinal.ID() {
			ancestor, err := b.clusterHeaders.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
			}

			if ancestor.Height <= clusterFinal.Height {
				return fmt.Errorf("should always build on last finalized block")
			}

			payload, err := b.payloads.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
			}

			collection := payload.Collection
			for _, tx := range collection.Transactions {
				unfinalizedLookup[tx.ID()] = struct{}{}
			}
			ancestorID = ancestor.ParentID
		}

		//TODO for now we check at most 1000 finalized ancestors - we should
		// instead look back based on reference block ID and expiry
		// ref: https://github.com/dapperlabs/flow-go/issues/3556
		limit := clusterFinal.Height - 1000
		if clusterFinal.Height < 1000 { // check underflow
			limit = 0
		}

		// look up previously included transactions in FINALIZED ancestors
		finalizedLookup := make(map[flow.Identifier]struct{})
		ancestorID = clusterFinal.ID()
		ancestorHeight := clusterFinal.Height
		for ancestorHeight > limit {
			ancestor, err := b.clusterHeaders.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not get ancestor header (%x): %w", ancestorID, err)
			}
			payload, err := b.payloads.ByBlockID(ancestorID)
			if err != nil {
				return fmt.Errorf("could not get ancestor payload (%x): %w", ancestorID, err)
			}

			collection := payload.Collection
			for _, tx := range collection.Transactions {
				finalizedLookup[tx.ID()] = struct{}{}
			}

			ancestorID = ancestor.ParentID
			ancestorHeight = ancestor.Height
		}

		// STEP THREE: build a payload of valid transactions, while at the same
		// time figuring out the correct reference block ID for the collection.

		minRefHeight := uint64(math.MaxUint64)
		// start with the finalized reference ID (longest expiry time)
		minRefID := refChainFinalizedID

		var transactions []*flow.TransactionBody
		for _, tx := range b.transactions.All() {

			// if we have reached maximum number of transactions, stop
			if uint(len(transactions)) >= b.maxCollectionSize {
				break
			}

			// retrieve the main chain header that was used as reference
			refHeader, err := b.mainHeaders.ByBlockID(tx.ReferenceBlockID)
			if err != nil {
				return fmt.Errorf("could not retrieve reference header: %w", err)
			}

			// for now, disallow un-finalized reference blocks
			if refChainFinalizedHeight < refHeader.Height {
				continue
			}

			// ensure the reference block is not too old
			txID := tx.ID()
			if refChainFinalizedHeight-refHeader.Height > uint64(flow.DefaultTransactionExpiry-b.expiryBuffer) {
				// the transaction is expired, it will never be valid
				b.transactions.Rem(txID)
				continue
			}

			// check that the transaction was not already used in un-finalized history
			_, duplicated := unfinalizedLookup[txID]
			if duplicated {
				continue
			}

			// check that the transaction was not already included in finalized history.
			_, duplicated = finalizedLookup[txID]
			if duplicated {
				// remove from mempool, conflicts with finalized block will never be valid
				b.transactions.Rem(txID)
				continue
			}

			// ensure we find the lowest reference block height
			if refHeader.Height < minRefHeight {
				minRefHeight = refHeader.Height
				minRefID = tx.ReferenceBlockID
			}

			transactions = append(transactions, tx)
		}

		// STEP FOUR: we have a set of transactions that are valid to include
		// on this fork. Now we need to create the collection that will be
		// used in the payload and construct the final proposal model

		// build the payload from the transactions
		payload := cluster.PayloadFromTransactions(minRefID, transactions...)

		header := flow.Header{
			ChainID:     parent.ChainID,
			ParentID:    parentID,
			Height:      parent.Height + 1,
			PayloadHash: payload.Hash(),
			Timestamp:   time.Now().UTC(),

			// NOTE: we rely on the HotStuff-provided setter to set the other
			// fields, which are related to signatures and HotStuff internals
		}

		// set fields specific to the consensus algorithm
		err = setter(&header)
		if err != nil {
			return fmt.Errorf("could not set fields to header: %w", err)
		}

		proposal = cluster.Block{
			Header:  &header,
			Payload: &payload,
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("could not build block: %w", err)
	}

	// finally we insert the block in a write transaction
	err = operation.RetryOnConflict(b.db.Update, procedure.InsertClusterBlock(&proposal))
	if err != nil {
		return nil, fmt.Errorf("could not insert built block: %w", err)
	}

	return proposal.Header, err
}

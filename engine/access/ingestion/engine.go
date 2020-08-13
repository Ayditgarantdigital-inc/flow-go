// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package ingestion

import (
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/access/rpc"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine represents the ingestion engine, used to funnel data from other nodes
// to a centralized location that can be queried by a user
type Engine struct {
	unit    *engine.Unit     // used to manage concurrency & shutdown
	log     zerolog.Logger   // used to log relevant actions with context
	state   protocol.State   // used to access the  protocol state
	me      module.Local     // used to access local node information
	request module.Requester // used to request collections

	// storage
	// FIX: remove direct DB access by substituting indexer module
	blocks       storage.Blocks
	headers      storage.Headers
	collections  storage.Collections
	transactions storage.Transactions

	// metrics
	transactionMetrics         module.TransactionMetrics
	collectionsToMarkFinalized *stdmap.Times
	collectionsToMarkExecuted  *stdmap.Times
	blocksToMarkExecuted       *stdmap.Times

	rpcEngine *rpc.Engine
}

// New creates a new access ingestion engine
func New(
	log zerolog.Logger,
	net module.Network,
	state protocol.State,
	me module.Local,
	request module.Requester,
	blocks storage.Blocks,
	headers storage.Headers,
	collections storage.Collections,
	transactions storage.Transactions,
	transactionMetrics module.TransactionMetrics,
	collectionsToMarkFinalized *stdmap.Times,
	collectionsToMarkExecuted *stdmap.Times,
	blocksToMarkExecuted *stdmap.Times,
	rpcEngine *rpc.Engine,
) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	eng := &Engine{
		unit:                       engine.NewUnit(),
		log:                        log.With().Str("engine", "ingestion").Logger(),
		state:                      state,
		me:                         me,
		request:                    request,
		blocks:                     blocks,
		headers:                    headers,
		collections:                collections,
		transactions:               transactions,
		transactionMetrics:         transactionMetrics,
		collectionsToMarkFinalized: collectionsToMarkFinalized,
		collectionsToMarkExecuted:  collectionsToMarkExecuted,
		blocksToMarkExecuted:       blocksToMarkExecuted,
		rpcEngine:                  rpcEngine,
	}

	// register engine with the execution receipt provider
	_, err := net.Register(engine.ReceiveReceipts, eng)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
	}

	return eng, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the ingestion engine, we consider the engine up and running
// upon initialization.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the ingestion engine, it only waits for all submit goroutines to end.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// SubmitLocal submits an event originating on the local node.
func (e *Engine) SubmitLocal(event interface{}) {
	e.Submit(e.me.NodeID(), event)
}

// Submit submits the given event from the node with the given origin ID
// for processing in a non-blocking manner. It returns instantly and logs
// a potential processing error internally when done.
func (e *Engine) Submit(originID flow.Identifier, event interface{}) {
	e.unit.Launch(func() {
		err := e.process(originID, event)
		if err != nil {
			engine.LogError(e.log, err)
		}
	})
}

// ProcessLocal processes an event originating on the local node.
func (e *Engine) ProcessLocal(event interface{}) error {
	return e.Process(e.me.NodeID(), event)
}

// Process processes the given event from the node with the given origin ID in
// a blocking manner. It returns the potential processing error when done.
func (e *Engine) Process(originID flow.Identifier, event interface{}) error {
	return e.unit.Do(func() error {
		return e.process(originID, event)
	})
}

// process processes the given ingestion engine event. Events that are given
// to this function originate within the expulsion engine on the node with the
// given origin ID.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch entity := event.(type) {
	case *flow.ExecutionReceipt:
		return e.handleExecutionReceipt(originID, entity)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// OnFinalizedBlock is called by the follower engine after a block has been finalized and the state has been updated
func (e *Engine) OnFinalizedBlock(hb *model.Block) {
	e.unit.Launch(func() {
		blockID := hb.BlockID
		err := e.processFinalizedBlock(blockID)
		if err != nil {
			e.log.Error().Err(err).Hex("block_id", blockID[:]).Msg("failed to process block")
			return
		}
		e.trackFinalizedMetricForBlock(hb)
	})
}

// processBlock handles an incoming finalized block.
func (e *Engine) processFinalizedBlock(blockID flow.Identifier) error {

	block, err := e.blocks.ByID(blockID)
	if err != nil {
		return fmt.Errorf("failed to lookup block: %w", err)
	}

	// Notify rpc handler of new finalized block height
	e.rpcEngine.SubmitLocal(block)

	// FIX: we can't index guarantees here, as we might have more than one block
	// with the same collection as long as it is not finalized

	// TODO: substitute an indexer module as layer between engine and storage

	// index the block storage with each of the collection guarantee
	err = e.blocks.IndexBlockForCollections(block.Header.ID(), flow.GetIDs(block.Payload.Guarantees))
	if err != nil {
		return fmt.Errorf("could not index block for collections: %w", err)
	}

	// queue requesting each of the collections from the collection node
	for _, guarantee := range block.Payload.Guarantees {
		e.request.EntityByID(guarantee.ID(), filter.HasNodeID(guarantee.SignerIDs...))
	}

	return nil
}

func (e *Engine) trackFinalizedMetricForBlock(hb *model.Block) {
	// retrieve the block
	block, err := e.blocks.ByID(hb.BlockID)
	if err != nil {
		e.log.Warn().Err(err).Msg("could not track tx finalized metric: finalized block not found locally")
		return
	}

	// TODO lookup actual finalization time by looking at the block finalizing `b`
	now := time.Now().UTC()

	// mark all transactions as finalized
	// TODO: sample to reduce performance overhead
	for _, g := range block.Payload.Guarantees {
		l, err := e.collections.LightByID(g.CollectionID)
		if errors.Is(err, storage.ErrNotFound) {
			e.collectionsToMarkFinalized.Add(g.CollectionID, now)
			continue
		} else if err != nil {
			e.log.Warn().Err(err).Str("collection_id", g.CollectionID.String()).
				Msg("could not track tx finalized metric: finalized collection not found locally")
			continue
		}

		for _, t := range l.Transactions {
			e.transactionMetrics.TransactionFinalized(t, now)
		}
	}

	if ti, found := e.blocksToMarkExecuted.ByID(hb.BlockID); found {
		e.trackExecutedMetricForBlock(block, ti)
		e.blocksToMarkExecuted.Rem(hb.BlockID)
	}
}

func (e *Engine) handleExecutionReceipt(originID flow.Identifier, r *flow.ExecutionReceipt) error {
	e.trackExecutedMetricForReceipt(r)
	return nil
}

func (e *Engine) trackExecutedMetricForReceipt(r *flow.ExecutionReceipt) {
	// TODO add actual execution time to execution receipt?
	now := time.Now().UTC()

	// retrieve the block
	b, err := e.blocks.ByID(r.ExecutionResult.BlockID)
	if errors.Is(err, storage.ErrNotFound) {
		e.blocksToMarkExecuted.Add(r.ExecutionResult.BlockID, now)
		return
	} else if err != nil {
		e.log.Warn().Err(err).Msg("could not track tx executed metric: executed block not found locally")
		return
	}
	e.trackExecutedMetricForBlock(b, now)
}

func (e *Engine) trackExecutedMetricForBlock(block *flow.Block, ti time.Time) {

	// mark all transactions as executed
	// TODO: sample to reduce performance overhead
	for _, g := range block.Payload.Guarantees {
		l, err := e.collections.LightByID(g.CollectionID)
		if errors.Is(err, storage.ErrNotFound) {
			e.collectionsToMarkExecuted.Add(g.CollectionID, ti)
			continue
		} else if err != nil {
			e.log.Warn().Err(err).Str("collection_id", g.CollectionID.String()).
				Msg("could not track tx executed metric: executed collection not found locally")
			continue
		}

		for _, t := range l.Transactions {
			e.transactionMetrics.TransactionExecuted(t, ti)
		}
	}
}

// handleCollection handles the response of the a collection request made earlier when a block was received
func (e *Engine) handleCollection(originID flow.Identifier, entity flow.Entity) error {

	// convert the entity to a strictly typed collection
	collection, ok := entity.(*flow.Collection)
	if !ok {
		return fmt.Errorf("invalid entity type (%T)", entity)
	}

	light := collection.Light()

	if ti, found := e.collectionsToMarkFinalized.ByID(light.ID()); found {
		for _, t := range light.Transactions {
			e.transactionMetrics.TransactionFinalized(t, ti)
		}
		e.collectionsToMarkFinalized.Rem(light.ID())
	}

	if ti, found := e.collectionsToMarkExecuted.ByID(light.ID()); found {
		for _, t := range light.Transactions {
			e.transactionMetrics.TransactionExecuted(t, ti)
		}
		e.collectionsToMarkExecuted.Rem(light.ID())
	}

	// FIX: we can't index guarantees here, as we might have more than one block
	// with the same collection as long as it is not finalized

	// store the light collection (collection minus the transaction body - those are stored separately)
	// and add transaction ids as index
	err := e.collections.StoreLightAndIndexByTransaction(&light)
	if err != nil {
		// ignore collection if already seen
		if errors.Is(err, storage.ErrAlreadyExists) {
			e.log.Debug().
				Hex("collection_id", logging.Entity(light)).
				Msg("collection is already seen")
			return nil
		}
		return err
	}

	// now store each of the transaction body
	for _, tx := range collection.Transactions {
		err := e.transactions.Store(tx)
		if err != nil {
			return fmt.Errorf("could not store transaction (%x): %w", tx.ID(), err)
		}
	}

	return nil
}

func (e *Engine) OnCollection(originID flow.Identifier, entity flow.Entity) {
	err := e.handleCollection(originID, entity)
	if err != nil {
		e.log.Error().Err(err).Msg("could not handle collection")
		return
	}
}

// OnBlockIncorporated is a noop for this engine since access node is only dealing with finalized blocks
func (e *Engine) OnBlockIncorporated(*model.Block) {
}

// OnDoubleProposeDetected is a noop for this engine since access node is only dealing with finalized blocks
func (e *Engine) OnDoubleProposeDetected(*model.Block, *model.Block) {
}

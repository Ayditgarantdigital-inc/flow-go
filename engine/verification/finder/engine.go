package finder

import (
	"context"
	"fmt"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/module/trace"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

type Engine struct {
	unit               *engine.Unit
	log                zerolog.Logger
	metrics            module.VerificationMetrics
	tracer             module.Tracer
	me                 module.Local
	match              network.Engine
	receipts           mempool.PendingReceipts // used to keep the receipts as mempool
	headerStorage      storage.Headers         // used to check block existence before verifying
	processedResult    mempool.Identifiers     // used to keep track of the processed results
	receiptIDsByBlock  mempool.IdentifierMap   // used as a mapping to keep track of receipts associated with a block
	receiptIDsByResult mempool.IdentifierMap   // used as a mapping to keep track of receipts with the same result
}

func New(
	log zerolog.Logger,
	metrics module.VerificationMetrics,
	tracer module.Tracer,
	net module.Network,
	me module.Local,
	match network.Engine,
	receipts mempool.PendingReceipts,
	headerStorage storage.Headers,
	processedResults mempool.Identifiers,
	receiptsByBlock mempool.IdentifierMap,
	receiptsByResult mempool.IdentifierMap,
) (*Engine, error) {
	e := &Engine{
		unit:               engine.NewUnit(),
		log:                log.With().Str("engine", "finder").Logger(),
		metrics:            metrics,
		tracer:             tracer,
		me:                 me,
		match:              match,
		headerStorage:      headerStorage,
		receipts:           receipts,
		processedResult:    processedResults,
		receiptIDsByBlock:  receiptsByBlock,
		receiptIDsByResult: receiptsByResult,
	}

	_, err := net.Register(engine.ExecutionReceiptProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register engine on execution receipt provider channel: %w", err)
	}
	return e, nil
}

// Ready returns a channel that is closed when the verifier engine is ready.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a channel that is closed when the verifier engine is done.
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
		err := e.Process(originID, event)
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

// process receives and submits an event to the finder engine for processing.
// It returns an error so the finder engine will not propagate an event unless
// it is successfully processed by the engine.
// The origin ID indicates the node which originally submitted the event to
// the peer-to-peer network.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch resource := event.(type) {
	case *flow.ExecutionReceipt:
		return e.handleExecutionReceipt(originID, resource)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// handleExecutionReceipt receives an execution receipt and adds it to receipts mempool if all of following
// conditions are satisfied:
// - It has not yet been added to the mempool
func (e *Engine) handleExecutionReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {
	span, ok := e.tracer.GetSpan(receipt.ID(), trace.VERProcessExecutionReceipt)
	ctx := context.Background()
	if !ok {
		span = e.tracer.StartSpan(receipt.ID(), trace.VERProcessExecutionReceipt)
		span.SetTag("execution_receipt_id", receipt.ID())
		defer span.Finish()
	}
	ctx = opentracing.ContextWithSpan(ctx, span)
	childSpan, _ := e.tracer.StartSpanFromContext(ctx, trace.VERFindHandleExecutionReceipt)
	defer childSpan.Finish()

	receiptID := receipt.ID()
	resultID := receipt.ExecutionResult.ID()

	log := e.log.With().
		Str("engine", "finder").
		Hex("origin_id", logging.ID(originID)).
		Hex("receipt_id", logging.ID(receiptID)).
		Hex("result_id", logging.ID(resultID)).Logger()
	log.Info().Msg("execution receipt arrived")

	// monitoring: increases number of received execution receipts
	e.metrics.OnExecutionReceiptReceived()

	// checks if the result has already been handled
	if e.processedResult.Has(resultID) {
		log.Debug().Msg("drops handling already processed result")
		return nil
	}

	// adds the execution receipt in the mempool
	pr := &verification.PendingReceipt{
		Receipt:  receipt,
		OriginID: originID,
	}
	added := e.receipts.Add(pr)
	if !added {
		log.Debug().Msg("drops adding duplicate receipt")
		return nil
	}

	// records the execution receipt id based on its result id
	_, err := e.receiptIDsByResult.Append(resultID, receiptID)
	if err != nil {
		log.Debug().Err(err).Msg("could not add receipt id to receipt-ids-by-result mempool")
	}

	log.Info().Msg("execution receipt successfully handled")

	// checks receipt being processable
	e.checkReceipts([]context.Context{ctx}, []*verification.PendingReceipt{pr})

	return nil
}

// To implement FinalizationConsumer
func (e *Engine) OnBlockIncorporated(*model.Block) {

}

// OnFinalizedBlock is part of implementing FinalizationConsumer interface
//
// OnFinalizedBlock notifications are produced by the Finalization Logic whenever
// a block has been finalized. They are emitted in the order the blocks are finalized.
// Prerequisites:
// Implementation must be concurrency safe; Non-blocking;
// and must handle repetition of the same events (with some processing overhead).
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	start := time.Now()

	// retrieves all receipts that are pending for this block
	erIDs, ok := e.receiptIDsByBlock.Get(block.BlockID)
	if !ok {
		// no pending receipt for this block
		return
	}
	// removes list of receipt ids for this block
	ok = e.receiptIDsByBlock.Rem(block.BlockID)
	if !ok {
		e.log.Debug().
			Hex("block_id", logging.ID(block.BlockID)).
			Msg("could not remove pending receipts from mempool")
	}

	// constructs list of receipts pending for this block
	ers := make([]*verification.PendingReceipt, 0, len(erIDs))
	ctxs := make([]context.Context, 0, len(erIDs))
	for _, erID := range erIDs {
		span, ok := e.tracer.GetSpan(erID, trace.VERProcessExecutionReceipt)
		ctx := context.Background()
		if !ok {
			span = e.tracer.StartSpan(erID, trace.VERProcessExecutionReceipt, opentracing.StartTime(start))
			span.SetTag("execution_receipt_id", erID)
			defer span.Finish()
		}
		ctx = opentracing.ContextWithSpan(ctx, span)
		childSpan, _ := e.tracer.StartSpanFromContext(ctx, trace.VERFindOnFinalizedBlock,
			opentracing.StartTime(start))
		defer childSpan.Finish()

		er, ok := e.receipts.Get(erID)
		if !ok {
			e.log.Debug().
				Hex("receipt_id", logging.ID(erID)).
				Msg("could not retrieve pending receipt")
			continue
		}
		ers = append(ers, er)
		ctxs = append(ctxs, ctx)
	}
	e.checkReceipts(ctxs, ers)
}

// To implement FinalizationConsumer
func (e *Engine) OnDoubleProposeDetected(*model.Block, *model.Block) {}

// isProcessable returns true if the block for execution result is available in the storage
// otherwise it returns false. In the current version, it checks solely against the block that
// contains the collection guarantee.
func (e *Engine) isProcessable(result *flow.ExecutionResult) bool {
	// checks existence of block that result points to
	_, err := e.headerStorage.ByBlockID(result.BlockID)
	return err == nil
}

// processResult submits the result to the match engine.
// originID is the identifier of the node that initially sends a receipt containing this result.
func (e *Engine) processResult(ctx context.Context, originID flow.Identifier, result *flow.ExecutionResult) error {
	span, _ := e.tracer.StartSpanFromContext(ctx, trace.VERFindProcessResult)
	defer span.Finish()

	resultID := result.ID()
	if e.processedResult.Has(resultID) {
		e.log.Debug().
			Hex("result_id", logging.ID(resultID)).
			Msg("result already processed")
		return nil
	}
	err := e.match.Process(originID, result)
	if err != nil {
		return fmt.Errorf("submission error to match engine: %w", err)
	}

	// monitoring: increases number of execution results sent
	e.metrics.OnExecutionResultSent()

	return nil
}

// onResultProcessed is called whenever a result is processed completely and
// is passed to the match engine. It marks the result as processed, and removes
// all receipts with the same result from mempool.
func (e *Engine) onResultProcessed(ctx context.Context, resultID flow.Identifier) {
	span, _ := e.tracer.StartSpanFromContext(ctx, trace.VERFindOnResultProcessed)
	defer span.Finish()

	log := e.log.With().
		Hex("result_id", logging.ID(resultID)).
		Logger()
	// marks result as processed
	added := e.processedResult.Add(resultID)
	if added {
		log.Debug().Msg("result marked as processed")
	}

	// extracts all receipt ids with this result
	prIDs, ok := e.receiptIDsByResult.Get(resultID)
	if !ok {
		log.Debug().Msg("could not retrieve receipt ids associated with this result")
	}

	// drops all receipts with the same result
	for _, prID := range prIDs {
		// removes receipt from mempool
		removed := e.receipts.Rem(prID)
		if removed {
			log.Debug().
				Hex("receipt_id", logging.ID(prID)).
				Msg("receipt with processed result cleaned up")
		}
	}
}

// checkReceipts receives a set of receipts and evaluates each of them
// against being processable. If a receipt is processable, it gets processed.
func (e *Engine) checkReceipts(ctxs []context.Context, receipts []*verification.PendingReceipt) {
	e.unit.Lock()
	defer e.unit.Unlock()

	for i, pr := range receipts {
		ctx := ctxs[i]
		var span opentracing.Span
		span, ctx = e.tracer.StartSpanFromContext(ctx, trace.VERFindCheckReceipts)
		defer span.Finish()

		receiptID := pr.Receipt.ID()
		resultID := pr.Receipt.ExecutionResult.ID()
		if e.isProcessable(&pr.Receipt.ExecutionResult) {
			// checks if result is ready to process
			err := e.processResult(ctx, pr.OriginID, &pr.Receipt.ExecutionResult)
			if err != nil {
				e.log.Error().
					Err(err).
					Hex("receipt_id", logging.ID(receiptID)).
					Hex("result_id", logging.ID(resultID)).
					Msg("could not process result")
				continue
			}

			// performs clean up
			e.onResultProcessed(ctx, resultID)
		} else {
			// receipt is not processable
			// keeps track of it in id map
			_, err := e.receiptIDsByBlock.Append(pr.Receipt.ExecutionResult.BlockID, receiptID)
			if err != nil {
				e.log.Error().
					Err(err).
					Hex("block_id", logging.ID(pr.Receipt.ExecutionResult.BlockID)).
					Hex("receipt_id", logging.ID(receiptID)).
					Msg("could not append receipt to receipt-ids-by-block mempool")
			}
		}
	}
}

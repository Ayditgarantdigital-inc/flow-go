// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package matching

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/mempool"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/logging"
)

// Engine is the propagation engine, which makes sure that new collections are
// propagated to the other consensus nodes on the network.
type Engine struct {
	unit      *engine.Unit      // used to control startup/shutdown
	log       zerolog.Logger    // used to log relevant actions with context
	state     protocol.State    // used to access the  protocol state
	me        module.Local      // used to access local node information
	results   storage.Results   // used to permanently store results
	receipts  mempool.Receipts  // holds collection guarantees in memory
	approvals mempool.Approvals // holds result approvals in memory
	seals     mempool.Seals     // holds block seals in memory
}

// New creates a new collection propagation engine.
func New(log zerolog.Logger, net module.Network, state protocol.State, me module.Local, results storage.Results, receipts mempool.Receipts, approvals mempool.Approvals, seals mempool.Seals) (*Engine, error) {

	// initialize the propagation engine with its dependencies
	e := &Engine{
		unit:      engine.NewUnit(),
		log:       log.With().Str("engine", "matching").Logger(),
		state:     state,
		me:        me,
		results:   results,
		receipts:  receipts,
		approvals: approvals,
		seals:     seals,
	}

	// register engine with the receipt provider
	_, err := net.Register(engine.ReceiptProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for results: %w", err)
	}

	// register engine with the approval provider
	_, err = net.Register(engine.ApprovalProvider, e)
	if err != nil {
		return nil, fmt.Errorf("could not register for approvals: %w", err)
	}

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For the propagation engine, we consider the engine up and running
// upon initialization.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a done channel that is closed once the engine has fully stopped.
// For the propagation engine, it closes the channel when all submit goroutines
// have ended.
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
			e.log.Error().Err(err).Msg("could not process submitted event")
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

// process processes events for the propagation engine on the consensus node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch entity := event.(type) {
	case *flow.ExecutionReceipt:
		return e.onReceipt(originID, entity)
	case *flow.ResultApproval:
		return e.onApproval(originID, entity)
	default:
		return fmt.Errorf("invalid event type (%T)", event)
	}
}

// onReceipt processes a new execution receipt.
func (e *Engine) onReceipt(originID flow.Identifier, receipt *flow.ExecutionReceipt) error {

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("receipt_id", logging.Entity(receipt)).
		Msg("execution receipt received")

	// get the identity of the origin node, so we can check if it's a valid
	// source for a execution receipt (usually execution nodes)
	id, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("could not get origin identity: %w", err)
	}

	// check that the origin is an execution node
	if id.Role != flow.RoleExecution {
		return fmt.Errorf("invalid origin node role (%s)", id.Role)
	}

	// store in the memory pool
	err = e.receipts.Add(receipt)
	if err != nil {
		return fmt.Errorf("could not store receipt: %w", err)
	}

	err = e.matchReceipt(receipt)
	if err != nil {
		return fmt.Errorf("could not match receipt: %w", err)
	}

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("receipt_id", logging.Entity(receipt)).
		Msg("execution receipt processed")

	return nil
}

// onApproval processes a new result approval.
func (e *Engine) onApproval(originID flow.Identifier, approval *flow.ResultApproval) error {

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("approval_id", logging.Entity(approval)).
		Msg("result approval received")

	// get the identity of the origin node, so we can check if it's a valid
	// source for a result approval (usually verification node)
	id, err := e.state.Final().Identity(originID)
	if err != nil {
		return fmt.Errorf("could not get origin identity: %w", err)
	}

	// check that the origin is a verification node
	if id.Role != flow.RoleVerification {
		return fmt.Errorf("invalid origin node role (%s)", id.Role)
	}

	// store in the memory pool
	err = e.approvals.Add(approval)
	if err != nil {
		return fmt.Errorf("could not store approval: %w", err)
	}

	err = e.matchApproval(approval)
	if err != nil {
		return fmt.Errorf("could not match approval: %w", err)
	}

	e.log.Info().
		Hex("origin_id", originID[:]).
		Hex("approval_id", logging.Entity(approval)).
		Msg("result approval processed")

	return nil
}

// matchReceipt will check if the given receipt can already be used to create a
// block seal.
func (e *Engine) matchReceipt(receipt *flow.ExecutionReceipt) error {

	// try to find a matching approval
	approval, err := e.approvals.ByResultID(receipt.ExecutionResult.ID())
	if errors.Is(err, mempool.ErrEntityNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not query approval: %w", err)
	}

	// create the block seal if we found a match
	err = e.createSeal(receipt, approval)
	if err != nil {
		return fmt.Errorf("could not create seal: %w", err)
	}

	return nil
}

// matchApproval will check if the given approval can be used to finalize a
// block seal.
func (e *Engine) matchApproval(approval *flow.ResultApproval) error {

	// try to find a matching receipt
	receipt, err := e.receipts.ByResultID(approval.ResultApprovalBody.ExecutionResultID)
	if errors.Is(err, mempool.ErrEntityNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not query receipt: %w", err)
	}

	// create the block seal if we found a match
	err = e.createSeal(receipt, approval)
	if err != nil {
		return fmt.Errorf("could not create seal: %w", err)
	}

	return nil
}

// createSeal will check the receipt against the approval and create a seal if
// valid, which is stored in the seal memory pool.
func (e *Engine) createSeal(receipt *flow.ExecutionReceipt, approval *flow.ResultApproval) error {

	// get the previous result ID
	previous, err := e.results.ByID(receipt.ExecutionResult.PreviousResultID)
	if errors.Is(err, storage.ErrNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not get previous result: %w", err)
	}

	// store the new result
	err = e.results.Store(&receipt.ExecutionResult)
	if err != nil {
		return fmt.Errorf("could not store result: %w", err)
	}

	// add the seal to the block seal memory pool
	seal := flow.Seal{
		BlockID:       receipt.ExecutionResult.BlockID,
		PreviousState: previous.FinalStateCommit,
		FinalState:    receipt.ExecutionResult.FinalStateCommit,
	}
	err = e.seals.Add(&seal)
	if err != nil {
		return fmt.Errorf("could not store seal: %v", err)
	}

	return nil
}

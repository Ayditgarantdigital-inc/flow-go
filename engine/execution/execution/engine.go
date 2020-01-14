package execution

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/storage"
)

// Engine manages execution of transactions
type Engine struct {
	unit        *engine.Unit
	log         zerolog.Logger
	con         network.Conduit
	me          module.Local
	receipts    network.Engine
	collections storage.Collections
	executor    executor.BlockExecutor
}

func New(
	log zerolog.Logger,
	net module.Network,
	me module.Local,
	receipts network.Engine,
	collections storage.Collections,
	executor executor.BlockExecutor,
) (*Engine, error) {

	e := Engine{
		unit:        engine.NewUnit(),
		log:         log,
		me:          me,
		receipts:    receipts,
		collections: collections,
		executor:    executor,
	}

	con, err := net.Register(engine.ExecutionExecution, &e)
	if err != nil {
		return nil, errors.Wrap(err, "could not register engine")
	}

	e.con = con

	return &e, nil
}

// Ready returns a channel that will close when the engine has
// successfully started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done returns a channel that will close when the engine has
// successfully stopped.
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

// process processes events for the execution engine on the execution node.
func (e *Engine) process(originID flow.Identifier, event interface{}) error {
	switch ev := event.(type) {
	case *flow.Block:
		return e.onBlock(ev)
	default:
		return errors.Errorf("invalid event type (%T)", event)
	}
}

// onBlock is triggered when this engine receives a new block.
//
// This function fetches the collections and transactions in the block and passes
// them to the block executor for execution.
func (e *Engine) onBlock(block *flow.Block) error {
	collections, err := e.getCollections(block.Guarantees)
	if err != nil {
		return fmt.Errorf("failed to load guarantees: %w", err)
	}

	transactions, err := e.getTransactions(collections)
	if err != nil {
		return fmt.Errorf("failed to load transactions: %w", err)
	}

	executableBlock := &executor.ExecutableBlock{
		Block: block,
		// TODO: replace with real logic from: https://github.com/dapperlabs/flow-go/pull/2028
		Collections: []*executor.ExecutableCollection{
			{
				Guarantee:    block.Guarantees[0],
				Transactions: transactions,
			},
		},
		// TODO: populate this field from previous block
		PreviousResultID: flow.ZeroID,
	}

	result, err := e.executor.ExecuteBlock(executableBlock)
	if err != nil {
		return fmt.Errorf("failed to execute block: %w", err)
	}

	// submit execution result to receipt engine
	e.receipts.SubmitLocal(result)

	return nil
}

func (e *Engine) getCollections(guarantees []*flow.CollectionGuarantee) ([]*flow.Collection, error) {
	collections := make([]*flow.Collection, len(guarantees))

	for i, guarantee := range guarantees {
		c, err := e.collections.ByID(guarantee.ID())
		if err != nil {
			return nil, fmt.Errorf("failed to load collection: %w", err)
		}

		collections[i] = c
	}

	return collections, nil
}

func (e *Engine) getTransactions(cols []*flow.Collection) ([]*flow.TransactionBody, error) {
	txCount := 0

	for _, c := range cols {
		txCount += len(c.Transactions)
	}

	transactions := make([]*flow.TransactionBody, txCount)

	i := 0

	for _, c := range cols {
		for _, tx := range c.Transactions {
			transactions[i] = &tx
			i++
		}
	}

	return transactions, nil
}

package computer

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
)

// A BlockComputer executes the transactions in a block.
type BlockComputer interface {
	ExecuteBlock(*execution.CompleteBlock, *state.View) (*execution.ComputationResult, error)
}

type blockComputer struct {
	vm virtualmachine.VirtualMachine
}

// NewBlockComputer creates a new block executor.
func NewBlockComputer(vm virtualmachine.VirtualMachine) BlockComputer {
	return &blockComputer{
		vm: vm,
	}
}

// ExecuteBlock executes a block and returns the resulting chunks.
func (e *blockComputer) ExecuteBlock(
	block *execution.CompleteBlock,
	stateView *state.View,
) (*execution.ComputationResult, error) {
	results, err := e.executeBlock(block, stateView)
	if err != nil {
		return nil, fmt.Errorf("failed to execute transactions: %w", err)
	}

	// TODO: compute block fees & reward payments

	return results, nil
}

func (e *blockComputer) executeBlock(
	block *execution.CompleteBlock,
	stateView *state.View,
) (*execution.ComputationResult, error) {

	blockCtx := e.vm.NewBlockContext(&block.Block.Header)

	collections := block.Collections()

	views := make([]*state.View, len(collections))

	for i, collection := range collections {

		collectionView := stateView.NewChild()

		err := e.executeCollection(i, blockCtx, collectionView, collection)
		if err != nil {
			return nil, fmt.Errorf("failed to execute collection: %w", err)
		}

		views[i] = collectionView

		stateView.ApplyDelta(collectionView.Delta())
	}

	return &execution.ComputationResult{
		CompleteBlock: block,
		StateViews:    views,
	}, nil
}

func (e *blockComputer) executeCollection(
	index int,
	blockCtx virtualmachine.BlockContext,
	collectionView *state.View,
	collection *execution.CompleteCollection,
) error {

	for _, tx := range collection.Transactions {
		txView := collectionView.NewChild()

		result, err := blockCtx.ExecuteTransaction(txView, tx)
		if err != nil {
			return fmt.Errorf("failed to execute transaction: %w", err)
		}

		if result.Succeeded() {
			collectionView.ApplyDelta(txView.Delta())
		}
	}

	return nil
}

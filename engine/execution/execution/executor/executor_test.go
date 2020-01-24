package executor_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/execution/executor"
	"github.com/dapperlabs/flow-go/engine/execution/execution/state"
	statemock "github.com/dapperlabs/flow-go/engine/execution/execution/state/mock"
	"github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine"
	vmmock "github.com/dapperlabs/flow-go/engine/execution/execution/virtualmachine/mock"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestBlockExecutor_ExecuteBlock(t *testing.T) {
	vm := new(vmmock.VirtualMachine)
	bc := new(vmmock.BlockContext)
	es := new(statemock.ExecutionState)

	exe := executor.NewBlockExecutor(vm, es)

	tx1 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	tx2 := flow.TransactionBody{
		Script: []byte("transaction { execute {} }"),
	}

	transactions := []*flow.TransactionBody{&tx1, &tx2}

	col := flow.Collection{Transactions: transactions}

	guarantee := flow.CollectionGuarantee{
		CollectionID: col.ID(),
		Signatures:   nil,
	}

	payload := flow.Payload{
		Guarantees: []*flow.CollectionGuarantee{&guarantee},
	}

	block := flow.Block{
		Header: flow.Header{
			Number: 42,
		},
		Payload: payload,
	}

	completeBlock := &execution.CompleteBlock{
		Block: &block,
		CompleteCollections: map[flow.Identifier]*execution.CompleteCollection{
			guarantee.ID(): {
				Guarantee:    &guarantee,
				Transactions: transactions,
			},
		},
	}

	vm.On("NewBlockContext", &block).Return(bc)

	bc.On("ExecuteTransaction", mock.Anything, mock.Anything).
		Return(&virtualmachine.TransactionResult{}, nil).
		Twice()

	es.On("StateCommitmentByBlockID", block.ParentID).
		Return(unittest.StateCommitmentFixture(), nil)

	es.On("NewView", mock.Anything).
		Return(
			state.NewView(func(key string) (bytes []byte, e error) {
				return nil, nil
			}))

	es.On("CommitDelta", mock.Anything).Return(nil, nil)
	es.On("PersistStateCommitment", block.ID(), mock.Anything).Return(nil)
	es.On("PersistChunkHeader", mock.Anything, mock.Anything).Return(nil)

	result, err := exe.ExecuteBlock(completeBlock)
	assert.NoError(t, err)
	assert.Len(t, result.Chunks, 1)

	chunk := result.Chunks[0]
	assert.EqualValues(t, 0, chunk.CollectionIndex)

	vm.AssertExpectations(t)
	bc.AssertExpectations(t)
	es.AssertExpectations(t)
}

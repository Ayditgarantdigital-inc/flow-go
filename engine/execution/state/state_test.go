package state_test

import (
	"context"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine/execution/state"
	ledger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/mocks"
	"github.com/onflow/flow-go/utils/unittest"
)

func prepareTest(f func(t *testing.T, es state.ExecutionState, l *ledger.Ledger)) func(*testing.T) {
	return func(t *testing.T) {
		unittest.RunWithBadgerDB(t, func(badgerDB *badger.DB) {
			metricsCollector := &metrics.NoopCollector{}
			diskWal := &fixtures.NoopWAL{}
			ls, err := ledger.NewLedger(diskWal, 100, metricsCollector, zerolog.Nop(), ledger.DefaultPathFinderVersion)
			require.NoError(t, err)

			ctrl := gomock.NewController(t)

			stateCommitments := mocks.NewMockCommits(ctrl)
			blocks := mocks.NewMockBlocks(ctrl)
			headers := mocks.NewMockHeaders(ctrl)
			collections := mocks.NewMockCollections(ctrl)
			events := mocks.NewMockEvents(ctrl)
			serviceEvents := mocks.NewMockEvents(ctrl)
			txResults := mocks.NewMockTransactionResults(ctrl)

			stateCommitment := ls.InitialState()

			stateCommitments.EXPECT().ByBlockID(gomock.Any()).Return(stateCommitment, nil)

			chunkDataPacks := new(storage.ChunkDataPacks)

			results := new(storage.ExecutionResults)
			receipts := new(storage.ExecutionReceipts)
			myReceipts := new(storage.MyExecutionReceipts)

			es := state.NewExecutionState(
				ls, stateCommitments, blocks, headers, collections, chunkDataPacks, results, receipts, myReceipts, events, serviceEvents, txResults, badgerDB, trace.NewNoopTracer(),
			)

			f(t, es, ls)
		})
	}
}

func TestExecutionStateWithTrieStorage(t *testing.T) {
	registerID1 := "fruit"

	registerID2 := "vegetable"

	t.Run("commit write and read new state", prepareTest(func(t *testing.T, es state.ExecutionState, l *ledger.Ledger) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(context.Background(), flow.Identifier{})
		assert.NoError(t, err)

		view1 := es.NewView(sc1)

		err = view1.Set(registerID1, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)
		err = view1.Set(registerID2, "", "", flow.RegisterValue("carrot"))
		assert.NoError(t, err)

		sc2, err := state.CommitDelta(l, view1.Delta(), sc1)
		assert.NoError(t, err)

		view2 := es.NewView(sc2)

		b1, err := view2.Get(registerID1, "", "")
		assert.NoError(t, err)
		b2, err := view2.Get(registerID2, "", "")
		assert.NoError(t, err)

		assert.Equal(t, flow.RegisterValue("apple"), b1)
		assert.Equal(t, flow.RegisterValue("carrot"), b2)
	}))

	t.Run("commit write and read previous state", prepareTest(func(t *testing.T, es state.ExecutionState, l *ledger.Ledger) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(context.Background(), flow.Identifier{})
		assert.NoError(t, err)

		view1 := es.NewView(sc1)

		err = view1.Set(registerID1, "", "", []byte("apple"))
		assert.NoError(t, err)
		sc2, err := state.CommitDelta(l, view1.Delta(), sc1)
		assert.NoError(t, err)

		// update value and get resulting state commitment
		view2 := es.NewView(sc2)
		err = view2.Set(registerID1, "", "", []byte("orange"))
		assert.NoError(t, err)

		sc3, err := state.CommitDelta(l, view2.Delta(), sc2)
		assert.NoError(t, err)

		// create a view for previous state version
		view3 := es.NewView(sc2)

		// create a view for new state version
		view4 := es.NewView(sc3)

		// fetch the value at both versions
		b1, err := view3.Get(registerID1, "", "")
		assert.NoError(t, err)

		b2, err := view4.Get(registerID1, "", "")
		assert.NoError(t, err)

		assert.Equal(t, flow.RegisterValue("apple"), b1)
		assert.Equal(t, flow.RegisterValue("orange"), b2)
	}))

	t.Run("commit delete and read new state", prepareTest(func(t *testing.T, es state.ExecutionState, l *ledger.Ledger) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(context.Background(), flow.Identifier{})
		assert.NoError(t, err)

		// set initial value
		view1 := es.NewView(sc1)
		err = view1.Set(registerID1, "", "", []byte("apple"))
		assert.NoError(t, err)
		err = view1.Set(registerID2, "", "", []byte("apple"))
		assert.NoError(t, err)

		sc2, err := state.CommitDelta(l, view1.Delta(), sc1)
		assert.NoError(t, err)

		// update value and get resulting state commitment
		view2 := es.NewView(sc2)
		err = view2.Delete(registerID1, "", "")
		assert.NoError(t, err)

		sc3, err := state.CommitDelta(l, view2.Delta(), sc2)
		assert.NoError(t, err)

		// create a view for previous state version
		view3 := es.NewView(sc2)

		// create a view for new state version
		view4 := es.NewView(sc3)

		// fetch the value at both versions
		b1, err := view3.Get(registerID1, "", "")
		assert.NoError(t, err)

		b2, err := view4.Get(registerID1, "", "")
		assert.NoError(t, err)

		assert.Equal(t, flow.RegisterValue("apple"), b1)
		assert.Empty(t, b2)
	}))

	t.Run("commit delta and persist state commit for the second time should be OK", prepareTest(func(t *testing.T, es state.ExecutionState, l *ledger.Ledger) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(context.Background(), flow.Identifier{})
		assert.NoError(t, err)

		// set initial value
		view1 := es.NewView(sc1)
		err = view1.Set(registerID1, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)
		err = view1.Set(registerID2, "", "", flow.RegisterValue("apple"))
		assert.NoError(t, err)

		sc2, err := state.CommitDelta(l, view1.Delta(), sc1)
		assert.NoError(t, err)

		// committing for the second time should be OK
		sc2Same, err := state.CommitDelta(l, view1.Delta(), sc1)
		assert.NoError(t, err)

		require.Equal(t, sc2, sc2Same)
	}))

}

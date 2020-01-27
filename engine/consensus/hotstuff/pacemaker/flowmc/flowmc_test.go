package flowmc

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff"
	mockdist "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications/mock"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/pacemaker/flowmc/timeout"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/stretchr/testify/assert"
)

const (
	startRepTimeout         float64 = 400.0
	minRepTimeout           float64 = 100.0
	voteTimeoutFraction     float64 = 0.5
	multiplicateiveIncrease float64 = 1.5
	additiveDecrease        float64 = 50
)

func initPaceMaker(t *testing.T, view uint64) (hotstuff.PaceMaker, *mockdist.Distributor) {
	notifier := &mockdist.Distributor{}
	tc, err := timeout.NewConfig(startRepTimeout, minRepTimeout, voteTimeoutFraction, multiplicateiveIncrease, additiveDecrease)
	if err != nil {
		t.Fail()
	}
	pm, err := New(view, timeout.NewController(*tc), notifier)
	if err != nil {
		t.Fail()
	}

	notifier.On("OnStartingBlockTimeout", uint64(3)).Return().Once()
	pm.Start()
	return pm, notifier
}

func qc(view uint64) *types.QuorumCertificate {
	return &types.QuorumCertificate{View: view}
}

func makeBlockProposal(qcView, blockView uint64) *types.BlockProposal {
	return &types.BlockProposal{
		Block: &types.Block{View: blockView, QC: qc(qcView)},
	}
}

// Test_SkipIncreaseViewThroughQC tests that PaceMaker increases View when receiving QC,
// if applicable, by skipping views
func Test_SkipIncreaseViewThroughQC(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)

	notifier.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	nve, nveOccured := pm.UpdateCurViewWithQC(qc(3))
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
	assert.True(t, nveOccured && nve.View == 4)

	notifier.On("OnSkippedAhead", uint64(13)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(13)).Return().Once()
	nve, nveOccured = pm.UpdateCurViewWithQC(qc(12))
	assert.True(t, nveOccured && nve.View == 13)

	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(13), pm.CurView())
	//notifier.AssertNumberOfCalls(t, "OnEnteringView", 1)
}

// Test_IgnoreOldBlocks tests that PaceMaker ignores old blocks
func Test_IgnoreOldQC(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)
	nve, nveOccured := pm.UpdateCurViewWithQC(qc(2))
	assert.True(t, !nveOccured && nve == nil)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(3), pm.CurView())
}

// Test_SkipViewThroughBlock tests that PaceMaker skips View when receiving Block containing QC with larger View Number
func Test_SkipViewThroughBlock(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)

	notifier.On("OnSkippedAhead", uint64(6)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(6)).Return().Once()
	nve, nveOccured := pm.UpdateCurViewWithBlock(makeBlockProposal(5, 9), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(6), pm.CurView())
	assert.True(t, nveOccured && nve.View == 6)

	notifier.On("OnSkippedAhead", uint64(23)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(23)).Return().Once()
	nve, nveOccured = pm.UpdateCurViewWithBlock(makeBlockProposal(22, 25), false)
	assert.True(t, nveOccured && nve.View == 23)

	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(23), pm.CurView())
}

// Test_HandlesSkipViewAttack verifies that PaceMaker skips views based on QC.view
// but NOT based on block.View to avoid vulnerability against Fast-Forward Attack
func Test_HandlesSkipViewAttack(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)

	notifier.On("OnSkippedAhead", uint64(6)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(6)).Return().Once()
	nve, nveOccured := pm.UpdateCurViewWithBlock(makeBlockProposal(5, 9), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(6), pm.CurView())
	assert.True(t, nveOccured && nve.View == 6)

	notifier.On("OnSkippedAhead", uint64(15)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(15)).Return().Once()
	nve, nveOccured = pm.UpdateCurViewWithBlock(makeBlockProposal(14, 23), false)
	assert.True(t, nveOccured && nve.View == 15)

	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(15), pm.CurView())
}

// Test_IgnoreOldBlocks tests that PaceMaker ignores old blocks
func Test_IgnoreOldBlocks(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)
	pm.UpdateCurViewWithBlock(makeBlockProposal(1, 2), false)
	pm.UpdateCurViewWithBlock(makeBlockProposal(1, 2), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(3), pm.CurView())
}

// Test_ProcessBlockForCurrentView tests that PaceMaker processes the block for the current view correctly
func Test_ProcessBlockForCurrentView(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)
	notifier.On("OnStartingVotesTimeout", uint64(3)).Return().Once()
	nve, nveOccured := pm.UpdateCurViewWithBlock(makeBlockProposal(1, 3), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(3), pm.CurView())
	assert.True(t, !nveOccured && nve == nil)

	pm, notifier = initPaceMaker(t, 3)
	notifier.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	nve, nveOccured = pm.UpdateCurViewWithBlock(makeBlockProposal(1, 3), false)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
	assert.True(t, nveOccured && nve.View == 4)
}

// Test_FutureBlockWithQcForCurrentView tests that PaceMaker processes the block with
//    block.qc.view = curView
//    block.view = curView +1
// correctly. Specifically, it should induce a view change to the block.view, which
// enables processing the block right away, i.e. switch to block.view + 1
func Test_FutureBlockWithQcForCurrentView(t *testing.T) {
	// NOT Primary for the Block's view
	pm, notifier := initPaceMaker(t, 3)
	notifier.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(5)).Return().Once()
	nve, nveOccured := pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), false)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(5), pm.CurView())
	assert.True(t, nveOccured && nve.View == 5)

	// PRIMARY for the Block's view
	pm, notifier = initPaceMaker(t, 3)
	notifier.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	notifier.On("OnStartingVotesTimeout", uint64(4)).Return().Once()
	nve, nveOccured = pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
	assert.True(t, nveOccured && nve.View == 4)
}

// Test_FutureBlockWithQcForCurrentView tests that PaceMaker processes the block with
//    block.qc.view > curView
//    block.view = block.qc.view +1
// correctly. Specifically, it should induce a view change to block.view, which
// enables processing the block right away, i.e. switch to block.view + 1
func Test_FutureBlockWithQcForFutureView(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)
	notifier.On("OnSkippedAhead", uint64(14)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(14)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(15)).Return().Once()
	nve, nveOccured := pm.UpdateCurViewWithBlock(makeBlockProposal(13, 14), false)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(15), pm.CurView())
	assert.True(t, nveOccured && nve.View == 15)

	pm, notifier = initPaceMaker(t, 3)
	notifier.On("OnSkippedAhead", uint64(14)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(14)).Return().Once()
	notifier.On("OnStartingVotesTimeout", uint64(14)).Return().Once()
	nve, nveOccured = pm.UpdateCurViewWithBlock(makeBlockProposal(13, 14), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(14), pm.CurView())
	assert.True(t, nveOccured && nve.View == 14)
}

// Test_FutureBlockWithQcForCurrentView tests that PaceMaker processes the block with
//    block.qc.view > curView
//    block.view > block.qc.view +1
// correctly. Specifically, it should induce a view change to the block.qc.view +1,
// which is not sufficient to process the block. I.e. we expect view change to block.qc.view +1
// enables processing the block right away.
func Test_FutureBlockWithQcForFutureFutureView(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3)
	notifier.On("OnSkippedAhead", uint64(14)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(14)).Return().Once()
	nve, nveOccured := pm.UpdateCurViewWithBlock(makeBlockProposal(13, 17), false)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(14), pm.CurView())
	assert.True(t, nveOccured && nve.View == 14)

	pm, notifier = initPaceMaker(t, 3)
	notifier.On("OnSkippedAhead", uint64(14)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(14)).Return().Once()
	nve, nveOccured = pm.UpdateCurViewWithBlock(makeBlockProposal(13, 17), true)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(14), pm.CurView())
	assert.True(t, nveOccured && nve.View == 14)
}


// Test_IgnoreBlockDuplicates tests that PaceMaker ignores duplicate blocks
func Test_IgnoreBlockDuplicates(t *testing.T) {
	// NOT Primary for the Block's view
	pm, notifier := initPaceMaker(t, 3)
	notifier.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(5)).Return().Once()
	nve, nveOccured := pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), false)
	assert.True(t, nveOccured && nve.View == 5)
	nve, nveOccured = pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), false)
	assert.True(t, !nveOccured && nve == nil)
	nve, nveOccured = pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), false)
	assert.True(t, !nveOccured && nve == nil)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(5), pm.CurView())

	// PRIMARY for the Block's view
	pm, notifier = initPaceMaker(t, 3)
	notifier.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	notifier.On("OnStartingVotesTimeout", uint64(4)).Return().Once()
	nve, nveOccured = pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), true)
	assert.True(t, nveOccured && nve.View == 4)
	nve, nveOccured = pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), true)
	assert.True(t, !nveOccured && nve == nil)
	nve, nveOccured = pm.UpdateCurViewWithBlock(makeBlockProposal(3, 4), true)
	assert.True(t, !nveOccured && nve == nil)
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
}

// Test_ReplicaTimeout tests that replica timeout fires as expected
func Test_ReplicaTimeout(t *testing.T) {
	start := time.Now()
	pm, notifier := initPaceMaker(t, 3) // initPaceMaker also calls Start() on PaceMaker

	select {
	case <-pm.TimeoutChannel():
		break // testing path: corresponds to EventLoop picking up timeout from channel
	case <-time.NewTimer(time.Duration(2) * time.Duration(startRepTimeout) * time.Millisecond).C:
		t.Fail() // to prevent test from hanging
	}
	duration := float64(time.Now().Sub(start).Milliseconds()) // in millisecond
	fmt.Println(duration)
	assert.True(t, math.Abs(duration-startRepTimeout) < 0.1*startRepTimeout)
	// While the timeout event has been put in the channel,
	// PaceMaker should NOT react on it without the timeout event being processed by the EventHandler
	assert.Equal(t, uint64(3), pm.CurView())

	// here the, the Event loop would now call EventHandler.OnTimeout() -> PaceMaker.OnTimeout()
	notifier.On("OnReachedBlockTimeout", uint64(3)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	toEvent, err := pm.OnTimeout(&types.Timeout{Mode: types.ReplicaTimeout, View: 3})
	if err != nil {
		assert.Fail(t, err.Error())
	}
	if toEvent == nil {
		assert.Fail(t, "Expecting ViewChange event as result of timeout")
	}

	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
}

// Test_VoteTimeout tests that vote timeout fires as expected
func Test_VoteTimeout(t *testing.T) {
	pm, notifier := initPaceMaker(t, 3) // initPaceMaker also calls Start() on PaceMaker
	start := time.Now()

	notifier.On("OnStartingVotesTimeout", uint64(3)).Return().Once()
	pm.UpdateCurViewWithBlock(makeBlockProposal(2, 3), true)
	notifier.AssertExpectations(t)

	expectedTimeout := startRepTimeout * voteTimeoutFraction
	select {
	case <-pm.TimeoutChannel():
		break // testing path: corresponds to EventLoop picking up timeout from channel
	case <-time.NewTimer(time.Duration(2) * time.Duration(expectedTimeout) * time.Millisecond).C:
		t.Fail() // to prevent test from hanging
	}
	duration := float64(time.Now().Sub(start).Milliseconds()) // in millisecond
	fmt.Println(duration)
	assert.True(t, math.Abs(duration-expectedTimeout) < 0.1*expectedTimeout)
	// While the timeout event has been put in the channel,
	// PaceMaker should NOT react on it without the timeout event being processed by the EventHandler
	assert.Equal(t, uint64(3), pm.CurView())

	// here the, the Event loop would now call EventHandler.OnTimeout() -> PaceMaker.OnTimeout()
	notifier.On("OnReachedVotesTimeout", uint64(3)).Return().Once()
	notifier.On("OnStartingBlockTimeout", uint64(4)).Return().Once()
	toEvent, err := pm.OnTimeout(&types.Timeout{Mode: types.VoteCollectionTimeout, View: 3})
	if err != nil {
		assert.Fail(t, err.Error())
	}
	if toEvent == nil {
		assert.Fail(t, "Expecting ViewChange event as result of timeout")
	}
	notifier.AssertExpectations(t)
	assert.Equal(t, uint64(4), pm.CurView())
}

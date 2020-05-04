package execution

import (
	"context"
	"fmt"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/ghost/client"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/integration/tests/common"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestExecutionChunkDataPacks(t *testing.T) {
	suite.Run(t, new(ExecutionSuite2))
}

type ExecutionSuite2 struct {
	suite.Suite
	common.TestnetStateTracker
	cancel  context.CancelFunc
	net     *testnet.FlowNetwork
	nodeIDs []flow.Identifier
	ghostID flow.Identifier
	exe1ID  flow.Identifier
	verID   flow.Identifier
}

func (gs *ExecutionSuite2) Ghost() *client.GhostClient {
	ghost := gs.net.ContainerByID(gs.ghostID)
	client, err := common.GetGhostClient(ghost)
	require.NoError(gs.T(), err, "could not get ghost client")
	return client
}

func (gs *ExecutionSuite2) AccessClient() *testnet.Client {
	client, err := testnet.NewClient(fmt.Sprintf(":%s", gs.net.AccessPorts[testnet.AccessNodeAPIPort]))
	require.NoError(gs.T(), err, "could not get access client")
	return client
}

func (gs *ExecutionSuite2) SetupTest() {

	// to collect node configs...
	var nodeConfigs []testnet.NodeConfig

	// need one access node
	acsConfig := testnet.NewNodeConfig(flow.RoleAccess)
	nodeConfigs = append(nodeConfigs, acsConfig)

	// generate the three consensus identities
	gs.nodeIDs = unittest.IdentifierListFixture(3)
	for _, nodeID := range gs.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(nodeID),
			testnet.WithLogLevel(zerolog.FatalLevel),
			testnet.WithAdditionalFlag("--hotstuff-timeout=12s"))
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	// need one execution nodes
	gs.exe1ID = unittest.IdentifierFixture()
	exe1Config := testnet.NewNodeConfig(flow.RoleExecution, testnet.WithID(gs.exe1ID),
		testnet.WithLogLevel(zerolog.InfoLevel))
	nodeConfigs = append(nodeConfigs, exe1Config)

	// need one verification node
	gs.verID = unittest.IdentifierFixture()
	verConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithID(gs.verID),
		testnet.WithLogLevel(zerolog.InfoLevel))
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one collection node
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithLogLevel(zerolog.FatalLevel))
	nodeConfigs = append(nodeConfigs, collConfig)

	// add the ghost node config
	gs.ghostID = unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleVerification, testnet.WithID(gs.ghostID), testnet.AsGhost(),
		testnet.WithLogLevel(zerolog.InfoLevel))
	nodeConfigs = append(nodeConfigs, ghostConfig)

	// generate the network config
	netConfig := testnet.NewNetworkConfig("execution_tests", nodeConfigs)

	// initialize the network
	gs.net = testnet.PrepareFlowNetwork(gs.T(), netConfig)

	// start the network
	ctx, cancel := context.WithCancel(context.Background())
	gs.cancel = cancel
	gs.net.Start(ctx)

	// start tracking blocks
	gs.Track(gs.T(), ctx, gs.Ghost())
}

func (gs *ExecutionSuite2) TearDownTest() {
	gs.net.Remove()
	gs.cancel()
}

func (gs *ExecutionSuite2) TestVerificationNodesRequestChunkDataPacks() {

	// wait for first finalized block, called blockA
	blockA := gs.BlockState.WaitForFirstFinalized(gs.T())
	gs.T().Logf("got blockA height %v ID %v", blockA.Header.Height, blockA.Header.ID())

	// wait for execution receipt for blockA from execution node 1
	erExe1BlockA := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockA.Header.ID(), gs.exe1ID)
	gs.T().Logf("got erExe1BlockA with SC %x", erExe1BlockA.ExecutionResult.FinalStateCommit)

	// assert there were no ChunkDataPackRequests from the verification node yet
	require.Equal(gs.T(), 0, gs.MsgState.LenFrom(gs.verID), "expected no ChunkDataPackRequest to be sent before a transaction existed")

	// send transaction
	err := common.DeployCounter(context.Background(), gs.AccessClient())
	require.NoError(gs.T(), err, "could not deploy counter")

	// wait until we see a different state commitment for a finalized block, call that block blockB
	blockB, _ := common.WaitUntilFinalizedStateCommitmentChanged(gs.T(), &gs.BlockState, &gs.ReceiptState)
	gs.T().Logf("got blockB height %v ID %v", blockB.Header.Height, blockB.Header.ID())

	// wait for execution receipt for blockB from execution node 1
	erExe1BlockB := gs.ReceiptState.WaitForReceiptFrom(gs.T(), blockB.Header.ID(), gs.exe1ID)
	gs.T().Logf("got erExe1BlockB with SC %x", erExe1BlockB.ExecutionResult.FinalStateCommit)

	// extract chunk ID from execution receipt
	require.Len(gs.T(), erExe1BlockB.ExecutionResult.Chunks, 1)
	chunkID := erExe1BlockB.ExecutionResult.Chunks[0].ID()

	// TODO the following is extremely flaky, investigate why and re-activate.
	// wait for ChunkDataPack pushed from execution node
	// msg := gs.MsgState.WaitForMsgFrom(gs.T(), common.MsgIsChunkDataPackResponse, gs.exe1ID)
	// chunkDataPackResponse := msg.(*messages.ChunkDataPackResponse)
	// require.Equal(gs.T(), erExe1BlockB.ExecutionResult.Chunks[0].ID(), chunkDataPackResponse.Data.ChunkID
	// TODO clear messages

	// send a ChunkDataPackRequest from Ghost node
	err = gs.Ghost().Send(context.Background(), engine.ChunkDataPackProvider, []flow.Identifier{gs.exe1ID}, &messages.ChunkDataPackRequest{ChunkID: chunkID})
	require.NoError(gs.T(), err)

	// wait for ChunkDataPackResponse
	msg2 := gs.MsgState.WaitForMsgFrom(gs.T(), common.MsgIsChunkDataPackResponse, gs.exe1ID)
	chunkDataPackResponse2 := msg2.(*messages.ChunkDataPackResponse)
	require.Equal(gs.T(), chunkID, chunkDataPackResponse2.Data.ChunkID)
	require.Equal(gs.T(), erExe1BlockB.ExecutionResult.Chunks[0].StartState, chunkDataPackResponse2.Data.StartState)
}

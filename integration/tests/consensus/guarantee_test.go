package consensus

import (
	"context"
	"testing"
	"time"

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

func TestCollectionGuaranteeCycle(t *testing.T) {
	suite.Run(t, new(GuaranteeSuite))
}

type GuaranteeSuite struct {
	suite.Suite
	cancel  context.CancelFunc
	net     *testnet.FlowNetwork
	nodeIDs []flow.Identifier
	ghostID flow.Identifier
	collID  flow.Identifier
}

func (gs *GuaranteeSuite) Start(timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	gs.cancel = cancel
	gs.net.Start(ctx)
}

func (gs *GuaranteeSuite) Stop() {
	err := gs.net.Stop()
	gs.cancel()
	require.NoError(gs.T(), err, "should stop without error")
	gs.net.Cleanup()
}

func (gs *GuaranteeSuite) Consensus(index int) *testnet.Container {
	require.True(gs.T(), index < len(gs.nodeIDs), "invalid node index (%d)", index)
	node, found := gs.net.ContainerByID(gs.nodeIDs[index])
	require.True(gs.T(), found, "could not find node")
	return node
}

func (gs *GuaranteeSuite) Ghost() *client.GhostClient {
	ghost, found := gs.net.ContainerByID(gs.ghostID)
	require.True(gs.T(), found, "could not find ghost containter")
	client, err := common.GetGhostClient(ghost)
	require.NoError(gs.T(), err, "could not get ghost client")
	return client
}

func (gs *GuaranteeSuite) SetupTest() {

	// to collect node configs...
	var nodeConfigs []testnet.NodeConfig

	// generate the three consensus identities
	gs.nodeIDs = unittest.IdentifierListFixture(3)

	// need one execution node
	exeConfig := testnet.NewNodeConfig(flow.RoleExecution)
	nodeConfigs = append(nodeConfigs, exeConfig)

	// need one verification node
	verConfig := testnet.NewNodeConfig(flow.RoleVerification)
	nodeConfigs = append(nodeConfigs, verConfig)

	// need one collection node
	gs.collID = unittest.IdentifierFixture()
	collConfig := testnet.NewNodeConfig(flow.RoleCollection, testnet.WithID(gs.collID))
	nodeConfigs = append(nodeConfigs, collConfig)

	// generate consensus node config for each consensus identity
	for _, nodeID := range gs.nodeIDs {
		nodeConfig := testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithID(nodeID))
		nodeConfigs = append(nodeConfigs, nodeConfig)
	}

	// add the ghost node config
	gs.ghostID = unittest.IdentifierFixture()
	ghostConfig := testnet.NewNodeConfig(flow.RoleAccess, testnet.WithID(gs.ghostID), testnet.AsGhost(true))
	nodeConfigs = append(nodeConfigs, ghostConfig)

	// generate the network config
	netConfig := testnet.NewNetworkConfig("consensus_collection_guarantee_cycle", nodeConfigs)

	// initialize the network
	gs.net = testnet.PrepareFlowNetwork(gs.T(), netConfig)
}

func (gs *GuaranteeSuite) TestCollectionGuaranteeIncluded() {

	// set timeout for loop
	timeout := time.Minute
	deadline := time.Now().Add(timeout)

	// start the network and defer cleanup
	gs.Start(timeout)

	// wait for 10 seconds before doing anything
	time.Sleep(10 * time.Second)

	// subscribe to block proposals
	reader, err := gs.Ghost().Subscribe(context.Background())
	require.NoError(gs.T(), err, "could not subscribe to ghost")

	// send a guarantee into the first consensus node
	guarantee := unittest.CollectionGuaranteeFixture()
	guarantee.SignerIDs = []flow.Identifier{gs.collID}
	err = gs.Ghost().Send(context.Background(), engine.CollectionProvider, gs.nodeIDs[:1], guarantee)
	require.NoError(gs.T(), err, "could not send collection guarantee")

	gs.T().Logf("looking for: %x", guarantee.CollectionID)

	// read messages until we see a block with this guarantee
	found := false
	for time.Now().Before(deadline) && !found {
		_, msg, err := reader.Next()
		require.NoError(gs.T(), err, "could not read next message")
		gs.T().Logf("%T", msg)
		proposal, ok := msg.(*messages.BlockProposal)
		if !ok {
			continue
		}
		gs.T().Logf("proposal: %x", proposal.Header.ID())
		for _, included := range proposal.Payload.Guarantees {
			gs.T().Logf("guarantee: %x", included.CollectionID)
			if included.CollectionID == guarantee.CollectionID {
				found = true
			}
		}
	}

	// stop the network
	gs.net.Stop()

	// make sure we found the guarantee
	require.True(gs.T(), found, "should have found guarantee in block")
}

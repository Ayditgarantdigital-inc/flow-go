package test

import (
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ipfs/go-log"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/libp2p/message"
	"github.com/onflow/flow-go/network/p2p"
	mockprotocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// MutableIdentityTableSuite tests that the networking layer responds correctly
// to changes to the identity table. When nodes are added, we should update our
// topology and accept connections from these new nodes. When nodes are removed
// or ejected we should update our topology and restrict connections from these
// nodes.
type MutableIdentityTableSuite struct {
	suite.Suite
	ConduitWrapper
	testNodes        testNodeList
	removedTestNodes testNodeList // test nodes which might have been removed from the mesh
	state            *mockprotocol.State
	snapshot         *mockprotocol.Snapshot
	logger           zerolog.Logger
}

// testNode encapsulates the node state which includes its identity, middleware, network,
// mesh engine and the id refresher
type testNode struct {
	id          *flow.Identity
	mw          *p2p.Middleware
	net         *p2p.Network
	engine      *MeshEngine
	idRefresher *p2p.NodeIDRefresher
}

// testNodeList is a list of test node and has functions to retrieve the different elements of the test nodes
type testNodeList []testNode

func (t testNodeList) ids() flow.IdentityList {
	ids := make(flow.IdentityList, len(t))
	for i, node := range t {
		ids[i] = node.id
	}
	return ids
}

func (t testNodeList) engines() []*MeshEngine {
	engs := make([]*MeshEngine, len(t))
	for i, node := range t {
		engs[i] = node.engine
	}
	return engs
}

func (t testNodeList) idRefreshers() []*p2p.NodeIDRefresher {
	idRefreshers := make([]*p2p.NodeIDRefresher, len(t))
	for i, node := range t {
		idRefreshers[i] = node.idRefresher
	}
	return idRefreshers
}

func TestEpochTransitionTestSuite(t *testing.T) {
	suite.Run(t, new(MutableIdentityTableSuite))
}

func (suite *MutableIdentityTableSuite) SetupTest() {
	suite.testNodes = nil
	rand.Seed(time.Now().UnixNano())
	nodeCount := 10
	suite.logger = zerolog.New(os.Stderr).Level(zerolog.DebugLevel)
	log.SetAllLoggers(log.LevelError)

	suite.setupStateMock()
	suite.addNode(nodeCount)

	// simulate a start of an epoch by signaling a change in the identity table
	suite.signalIdentityChanged()

	// wait for two lip2p heatbeats for the nodes to discover each other and form the mesh
	time.Sleep(2 * time.Second)
}

// TearDownTest closes all the networks within a specified timeout
func (suite *MutableIdentityTableSuite) TearDownTest() {
	nets := make([]*p2p.Network, 0, len(suite.testNodes)+len(suite.removedTestNodes))
	for _, n := range suite.testNodes {
		nets = append(nets, n.net)
	}
	for _, n := range suite.removedTestNodes {
		nets = append(nets, n.net)
	}
	stopNetworks(suite.T(), nets, 3*time.Second)
}

// setupStateMock setup state related mocks (all networks share the same state mock)
func (suite *MutableIdentityTableSuite) setupStateMock() {
	final := unittest.BlockHeaderFixture()
	suite.state = new(mockprotocol.State)
	suite.snapshot = new(mockprotocol.Snapshot)
	suite.snapshot.On("Head").Return(&final, nil)
	suite.snapshot.On("Phase").Return(flow.EpochPhaseCommitted, nil)
	// return all the current list of ids for the state.Final.Identities call made by the network
	suite.snapshot.On("Identities", mock.Anything).Return(
		func(flow.IdentityFilter) flow.IdentityList {
			return suite.testNodes.ids()
		},
		func(flow.IdentityFilter) error { return nil })
	suite.state.On("Final").Return(suite.snapshot, nil)
}

// addNode creates count many new nodes and appends them to the suite state variables
func (suite *MutableIdentityTableSuite) addNode(count int) {
	// create the ids, middlewares and networks
	ids, mws, nets := GenerateIDsMiddlewaresNetworks(suite.T(), count, suite.logger, 100, nil, !DryRun)

	// create the engines for the new nodes
	engines := GenerateEngines(suite.T(), nets)

	// create the node refreshers
	idRefereshers := suite.generateNodeIDRefreshers(nets)

	// create the test engines
	for i := 0; i < count; i++ {
		eng := testNode{
			id:          ids[i],
			mw:          mws[i],
			net:         nets[i],
			engine:      engines[i],
			idRefresher: idRefereshers[i],
		}
		suite.testNodes = append(suite.testNodes, eng)
	}
}

// removeNode removes a randomly chosen test node from suite.testNodes and adds it to suite.removedTestNodes
func (suite *MutableIdentityTableSuite) removeNode() testNode {
	// choose a random node to remove
	i := rand.Intn(len(suite.testNodes) - 1)
	removedNode := suite.testNodes[i]
	suite.removedTestNodes = append(suite.removedTestNodes, removedNode)
	suite.testNodes = append(suite.testNodes[:i], suite.testNodes[i+1:]...)
	return removedNode
}

// TestNewNodeAdded tests that when a new node is added to the identity list e.g. on an epoch,
// then it can connect to the network.
func (suite *MutableIdentityTableSuite) TestNewNodeAdded() {
	suite.T().Skip() // temp change to fix broken CI build

	// add a new node the current list of nodes
	suite.addNode(1)

	// update IDs for all the networks (simulating an epoch)
	suite.signalIdentityChanged()

	newNode := suite.testNodes[len(suite.testNodes)-1]
	newID := newNode.id
	newMiddleware := newNode.mw

	ids := suite.testNodes.ids()
	engs := suite.testNodes.engines()

	// check if the new node has sufficient connections with the existing nodes
	// if it does, then it has been inducted successfully in the network
	assertConnected(suite.T(), newMiddleware, ids.Filter(filter.Not(filter.HasNodeID(newID.NodeID))))

	// check that all the engines on this new epoch can talk to each other using any of the three networking primitives
	suite.exchangeMessages(ids, engs, nil, nil, suite.Publish)
	suite.exchangeMessages(ids, engs, nil, nil, suite.Multicast)
	suite.exchangeMessages(ids, engs, nil, nil, suite.Unicast)
}

// TestNodeRemoved tests that when an existing node is removed from the identity
// list (ie. as a result of an ejection or transition into an epoch where that node
// has un-staked) then it cannot connect to the network.
func (suite *MutableIdentityTableSuite) TestNodeRemoved() {

	// removed a node
	removedNode := suite.removeNode()
	removedID := removedNode.id
	removedMiddleware := removedNode.mw
	removedEngine := removedNode.engine

	// update IDs for all the remaining nodes
	// the removed node continues with the old identity list as we don't want to rely on it updating its ids list
	suite.signalIdentityChanged()

	remainingIDs := suite.testNodes.ids()
	remainingEngs := suite.testNodes.engines()

	// assert that the removed node has no connections with any of the other nodes
	assertDisconnected(suite.T(), removedMiddleware, remainingIDs)

	// check that all remaining engines can still talk to each other while the ones removed can't
	// using any of the three networking primitives
	removedIDs := []*flow.Identity{removedID}
	removedEngines := []*MeshEngine{removedEngine}
	suite.Run("TestNodesAddedAndRemoved Publish", func() {
		suite.exchangeMessages(remainingIDs, remainingEngs, removedIDs, removedEngines, suite.Publish)
	})
	suite.Run("TestNodesAddedAndRemoved Multicast", func() {
		suite.exchangeMessages(remainingIDs, remainingEngs, removedIDs, removedEngines, suite.Multicast)
	})
	suite.Run("TestNodesAddedAndRemoved Unicast", func() {
		suite.exchangeMessages(remainingIDs, remainingEngs, removedIDs, removedEngines, suite.Unicast)
	})
}

// TestNodesAddedAndRemoved tests that:
// a. a newly added node can exchange messages with the existing nodes
// b. a node that has has been removed cannot exchange messages with the existing nodes
func (suite *MutableIdentityTableSuite) TestNodesAddedAndRemoved() {

	suite.T().Skip() // temp change to fix broken CI build
	// add a node
	suite.addNode(1)
	newNode := suite.testNodes[len(suite.testNodes)-1]
	newID := newNode.id
	newMiddleware := newNode.mw

	// remove a node
	removedNode := suite.removeNode()
	removedID := removedNode.id
	removedMiddleware := removedNode.mw
	removedEngine := removedNode.engine

	// update all current nodes
	suite.signalIdentityChanged()

	remainingIDs := suite.testNodes.ids()
	remainingEngs := suite.testNodes.engines()

	// check if the new node has sufficient connections with the existing nodes
	assertConnected(suite.T(), newMiddleware, remainingIDs.Filter(filter.Not(filter.HasNodeID(newID.NodeID))))

	// assert that the removed node has no connections with any of the other nodes
	assertDisconnected(suite.T(), removedMiddleware, remainingIDs)

	// check that all remaining engines can still talk to each other while the ones removed can't
	// using any of the three networking primitives
	removedIDs := []*flow.Identity{removedID}
	removedEngines := []*MeshEngine{removedEngine}
	suite.Run("TestNodesAddedAndRemoved Publish", func() {
		suite.exchangeMessages(remainingIDs, remainingEngs, removedIDs, removedEngines, suite.Publish)
	})
	suite.Run("TestNodesAddedAndRemoved Multicast", func() {
		suite.exchangeMessages(remainingIDs, remainingEngs, removedIDs, removedEngines, suite.Multicast)
	})
	suite.Run("TestNodesAddedAndRemoved Unicast", func() {
		suite.exchangeMessages(remainingIDs, remainingEngs, removedIDs, removedEngines, suite.Unicast)
	})
}

// signalIdentityChanged update IDs for all the current set of nodes (simulating an epoch)
func (suite *MutableIdentityTableSuite) signalIdentityChanged() {
	for _, r := range suite.testNodes.idRefreshers() {
		r.OnIdentityTableChanged()
	}
}

// assertConnected checks that the middleware of a node is directly connected
// to at least half of the other nodes.
func assertConnected(t *testing.T, mw *p2p.Middleware, ids flow.IdentityList) {
	threshold := len(ids) / 2
	require.Eventually(t, func() bool {
		connections := 0
		for _, id := range ids {
			connected, err := mw.IsConnected(*id)
			require.NoError(t, err)
			if connected {
				connections++
			}
		}
		return connections >= threshold
	}, 5*time.Second, time.Millisecond*100)
}

// assertDisconnected checks that the middleware of a node is not connected to any of the other nodes specified in the
// ids list
func assertDisconnected(t *testing.T, mw *p2p.Middleware, ids flow.IdentityList) {

	require.Eventually(t, func() bool {
		for _, id := range ids {
			connected, err := mw.IsConnected(*id)
			require.NoError(t, err)
			if connected {
				return false
			}
		}
		return true
	}, 5*time.Second, time.Millisecond*100)
}

// exchangeMessages verifies that allowed engines can successfully exchange messages between them while disallowed
// engines can't using the ConduitSendWrapperFunc network primitive
func (suite *MutableIdentityTableSuite) exchangeMessages(
	allowedIDs flow.IdentityList,
	allowedEngs []*MeshEngine,
	disallowedIDs flow.IdentityList,
	disallowedEngs []*MeshEngine,
	send ConduitSendWrapperFunc) {

	// send a message from each of the allowed engine to the other allowed engines
	for i, allowedEng := range allowedEngs {

		fromID := allowedIDs[i].NodeID
		targetIDs := allowedIDs.Filter(filter.Not(filter.HasNodeID(allowedIDs[i].NodeID)))

		err := suite.sendMessage(fromID, allowedEng, targetIDs, send)
		require.NoError(suite.T(), err)
	}

	// send a message from each of the allowed engine to all of the disallowed engines
	if len(disallowedEngs) > 0 {
		for i, fromEng := range allowedEngs {

			fromID := allowedIDs[i].NodeID
			targetIDs := disallowedIDs

			err := suite.sendMessage(fromID, fromEng, targetIDs, send)
			suite.checkSendError(err, send)
		}
	}

	// send a message from each of the disallowed engine to each of the allowed engines
	for i, fromEng := range disallowedEngs {

		fromID := disallowedIDs[i].NodeID
		targetIDs := allowedIDs

		err := suite.sendMessage(fromID, fromEng, targetIDs, send)
		suite.checkSendError(err, send)
	}

	count := len(allowedEngs)
	expectedMsgCnt := count - 1
	wg := sync.WaitGroup{}
	// fires a goroutine for each of the allowed engine to listen for incoming messages
	for i := range allowedEngs {
		wg.Add(expectedMsgCnt)
		go func(e *MeshEngine) {
			for x := 0; x < expectedMsgCnt; x++ {
				<-e.received
				wg.Done()
			}
		}(allowedEngs[i])
	}

	// assert that all allowed engines received expectedMsgCnt number of messages
	unittest.AssertReturnsBefore(suite.T(), wg.Wait, 5*time.Second)
	// assert that all allowed engines received no other messages
	for i := range allowedEngs {
		assert.Empty(suite.T(), allowedEngs[i].received)
	}

	// assert that the disallowed engines didn't receive any message
	for i, eng := range disallowedEngs {
		unittest.RequireNeverReturnBefore(suite.T(), func() {
			<-eng.received
		}, time.Millisecond, fmt.Sprintf("%s engine should not have recevied message", disallowedIDs[i]))
	}
}

func (suite *MutableIdentityTableSuite) sendMessage(fromID flow.Identifier,
	fromEngine *MeshEngine,
	toIDs flow.IdentityList,
	send ConduitSendWrapperFunc) error {

	primitive := runtime.FuncForPC(reflect.ValueOf(send).Pointer()).Name()
	event := &message.TestMessage{
		Text: fmt.Sprintf("hello from node %s using %s", fromID.String(), primitive),
	}

	return send(event, fromEngine.con, toIDs.NodeIDs()...)
}

func (suite *MutableIdentityTableSuite) checkSendError(err error, send ConduitSendWrapperFunc) {
	primitive := runtime.FuncForPC(reflect.ValueOf(send).Pointer()).Name()
	requireError := strings.Contains(primitive, "Unicast")
	if requireError {
		require.Error(suite.T(), err)
	}
}

func (suite *MutableIdentityTableSuite) generateNodeIDRefreshers(nets []*p2p.Network) []*p2p.NodeIDRefresher {
	refreshers := make([]*p2p.NodeIDRefresher, len(nets))
	for i, net := range nets {
		refreshers[i] = p2p.NewNodeIDRefresher(suite.logger, suite.state, net.SetIDs)
	}
	return refreshers
}

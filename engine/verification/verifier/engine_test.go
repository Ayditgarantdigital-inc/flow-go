package verifier

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
)

// VerifierEngineTestSuit encloses functionality testings of Verifier Engine
type VerifierEngineTestSuit struct {
	suite.Suite
	net   *mock.Network      // used as an instance of networking layer for the mock engine
	state *protocol.State    // used as a mock protocol state of nodes for verification engine
	ss    *protocol.Snapshot // used as a mock representation of the snapshot of system (part of State)
	me    *mock.Local        // used as a mock representation of the mock verification node (owning the verifier engine)
	con   *network.Conduit   // used as a mock instance of conduit that connects engine to network
}

// TestVerifierEngineTestSuite is the constructor of the TestSuite of the verifier engine
// Invoking this method executes all the subsequent tests methods of VerifierEngineTestSuit type
func TestVerifierEngineTestSuite(t *testing.T) {
	suite.Run(t, new(VerifierEngineTestSuit))
}

// SetupTests initiates the test setups prior to each test
func (v *VerifierEngineTestSuit) SetupTest() {
	// initializing test suite fields
	v.state = &protocol.State{}
	v.con = &network.Conduit{}
	v.net = &mock.Network{}
	v.me = &mock.Local{}
	v.ss = &protocol.Snapshot{}

	// mocking the network registration of the engine
	// all subsequent tests are expected to have a call on Register method
	v.net.On("Register", uint8(engine.VerificationVerifier), testifymock.AnythingOfType("*verifier.Engine")).
		Return(v.con, nil).
		Once()
}

// TestEncodeResultApproval tests encoding of result approvals
// making sure that encoding works correctly
func (v *VerifierEngineTestSuit) TestEncodeResultApproval() {
	resApprove := flow.ResultApproval{}
	fp := resApprove.Fingerprint()
	require.NotNil(v.T(), fp, "nil fingerprint")
}

// TestNewEngine verifies the establishment of the network registration upon
// creation of an instance of verifier.Engine using the New method
// It also returns an instance of new engine to be used in the later tests
func (v *VerifierEngineTestSuit) TestNewEngine() *Engine {
	// creating a new engine
	e, err := New(zerolog.Logger{}, v.net, v.state, v.me)
	require.Nil(v.T(), err, "could not create an engine")
	v.net.AssertExpectations(v.T())
	return e
}

// TestProcessLocalHappyPath covers the happy path of submitting a valid execution receipt to
// a single verifier engine till a result approval is emitted to all the consensus nodes
func (v *VerifierEngineTestSuit) TestProcessLocalHappyPath() {
	// creating a new engine
	vrfy := v.TestNewEngine()
	//mocking the identity of the verification node under test
	vnMe := newMockVrfyID()

	//mocking state for me.NodeID for internal call in ProcessLocal method
	v.me.On("NodeID").Return(vnMe.NodeID).Once()

	// mocking for Final().Identities(Identity(originID)) in onExecutionReceipt method
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identity", vnMe.NodeID).Return(vnMe, nil).Once()

	//mocking state for me.NodeID for internal call in onExecutionReceipt method
	v.me.On("NodeID").Return(vnMe.NodeID).Once()

	// a set of mock staked consensus nodes
	consID := generateMockConsensusIDs(100)

	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus)) call in verify method
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identities", testifymock.Anything).Return(consID, nil)

	// generating a random ER and its associated result approval
	er := verification.RandomERGen()
	restApprov := verification.RandomRAGen(er)

	// the happy path ends by the verifier engine emitting a
	// result approval to ONLY all the consensus nodes
	// generating and mocking parameters of Submit method
	params := genSubmitParams(restApprov, consID)
	v.con.On("Submit", params...).
		Return(nil).
		Once()

	// store of the engine should be empty prior to the submit
	assert.Equal(v.T(), vrfy.store.ResultsNum(), 0)

	// emitting an execution receipt form the execution node
	_ = vrfy.ProcessLocal(er)

	// store of the engine should be of size one prior to the submit
	assert.Equal(v.T(), vrfy.store.ResultsNum(), 1)

	vrfy.wg.Wait()
	v.state.AssertExpectations(v.T())
	v.con.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
	v.me.AssertExpectations(v.T())
}

// TestProcessUnhappyInput covers unhappy inputs for Process method
// including nil event, empty event, and non-existing IDs
func (v *VerifierEngineTestSuit) TestProcessUnhappyInput() {
	// mocking state for Final().Identity(flow.Identifier{}) call in onExecutionReceipt
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identity", flow.Identifier{}).Return(flow.Identity{}, errors.New("non-nil")).Once()

	// creating a new engine
	vrfy := v.TestNewEngine()

	// nil event
	err := vrfy.Process(flow.Identifier{}, nil)
	assert.NotNil(v.T(), err, "failed recognizing nil event")

	// non-execution receipt event
	err = vrfy.Process(flow.Identifier{}, new(struct{}))
	assert.NotNil(v.T(), err, "failed recognizing non-execution receipt events")

	// non-recoverable id
	err = vrfy.Process(flow.Identifier{}, &flow.ExecutionReceipt{})
	assert.NotNilf(v.T(), err, "broken happy path: %s", err)

	// asserting the calls in unhappy path
	vrfy.wg.Wait()
	v.net.AssertExpectations(v.T())
	v.state.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
}

// TestProcessUnstakeEmit tests the Process method of Verifier engine against
// an unauthorized node emitting an execution receipt. The process method should
// catch this injected fault by returning an error
func (v *VerifierEngineTestSuit) TestProcessUnstakeEmit() {
	// creating a new engine
	vrfy := v.TestNewEngine()

	unstakedID := flow.Identity{
		NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
		Address: "unstaked_address",
		Role:    flow.RoleExecution,
		Stake:   0,
	}

	// mocking state for Final().Identity(unstakedID.NodeID) call in onExecutionReceipt
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identity", unstakedID.NodeID).
		Return(flow.Identity{}, errors.New("non-nil")).Once()

	// execution receipts should directly come from Execution Nodes,
	// hence for all test cases a non-nil error should returned
	err := vrfy.Process(unstakedID.NodeID, &flow.ExecutionReceipt{})
	assert.NotNil(v.T(), err, "failed rejecting an unstaked id")

	vrfy.wg.Wait()
	v.state.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
}

// TestProcessUnauthorizedEmits follows the unhappy path where staked nodes
// rather than execution nodes send an execution receipt event
func (v *VerifierEngineTestSuit) TestProcessUnauthorizedEmits() {
	// creating a new engine
	vrfy := v.TestNewEngine()

	// defining mock nodes identities
	// test table covers all roles except the execution nodes
	// that are the only legitimate party to originate an execution receipt
	tt := []struct {
		role flow.Role //the test input
		err  error     //expected test result
	}{
		{ // consensus node
			role: flow.RoleConsensus,
			err:  errors.New("non-nil"),
		},
		{ // observer node
			role: flow.RoleObservation,
			err:  errors.New("non-nil"),
		},
		{ // collection node
			role: flow.RoleCollection,
			err:  errors.New("non-nil"),
		},
		{ // verification node
			role: flow.RoleVerification,
			err:  errors.New("non-nil"),
		},
	}

	// mocking the identity of the verification node under test
	vnMe := newMockVrfyID()

	for _, tc := range tt {
		id := flow.Identity{
			NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
			Address: "mock-address",
			Role:    tc.role,
		}
		// mocking state for Final().Identity(originID) call in onExecutionReceipt method
		v.state.On("Final").Return(v.ss).Once()
		v.ss.On("Identity", id.NodeID).Return(id, nil).Once()

		//mocking state for e.me.NodeID(vnMe.NodeID) call in onExecutionReceipt method
		v.me.On("NodeID").Return(vnMe.NodeID).Once()

		// execution receipts should directly come from Execution Nodes,
		// hence for all test cases a non-nil error should returned
		err := vrfy.Process(id.NodeID, &flow.ExecutionReceipt{})
		assert.NotNil(v.T(), err, "failed rejecting an faulty origin id")

		vrfy.wg.Wait()
		v.state.AssertExpectations(v.T())
		v.ss.AssertExpectations(v.T())
		v.me.AssertExpectations(v.T())
	}
}

// ConcurrencyTestSetup is a sub-test method. It is not invoked independently, rather
// it is executed as part of other test methods.
// It does the followings:
// 1- creates and returns a mock verifier engine as part of return values
// 2- creates and returns a mock staked execution node ID as part of return values
// 3- It receives the concurrency degree as input and mocks the methods to expect calls equal to that degree
// 4- It generates and mocks a consensus committee for the verifier engine to contact, and mocks
// Submit method of the verifier engine.
// 5- It generates a valid execution receipt and mocks the verifier node accept that from the generated execution node ID
// in step (2), and emit a result approval to the consensus committee generated in step (4).
func (v *VerifierEngineTestSuit) ConcurrencyTestSetup(degree, consNum int) (*flow.Identity, *Engine, *flow.ExecutionReceipt) {
	// creating a new engine
	vrfy := v.TestNewEngine()

	// a mock staked execution node for generating a mock execution receipt
	exeID := flow.Identity{
		NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
		Address: "mock-en-address",
		Role:    flow.RoleExecution,
	}

	// mocking state fo Final().Identity(originID) call in onExecutionReceipt method
	v.state.On("Final").Return(v.ss).Times(degree)
	v.ss.On("Identity", exeID.NodeID).Return(exeID, nil).Times(degree)

	consIDs := generateMockConsensusIDs(consNum)

	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus)) call in verify method
	// since all ERs are the same, only one call for identity of consensus nodes should happen
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identities", testifymock.Anything).Return(consIDs, nil)

	// generating a random execution receipt and its corresponding result approval
	er := verification.RandomERGen()
	restApprov := verification.RandomRAGen(er)

	// the happy path ends by the verifier engine emitting a
	// result approval to ONLY all the consensus nodes
	// since all ERs are the same, only one submission should happen
	// generating and mocking parameters of Submit method
	params := genSubmitParams(restApprov, consIDs)
	v.con.On("Submit", params...).
		Return(nil).
		Once()

	return &exeID, vrfy, er
}

// TestProcessHappyPathConcurrentERs covers the happy path of the verifier engine on concurrently
// receiving a valid execution receipt several times. The execution receipts are coming from a single
// execution node. The expected behavior is to verify only a single copy of those receipts while dismissing the rest.
func (v *VerifierEngineTestSuit) TestProcessHappyPathConcurrentERs() {
	// ConcurrencyDegree defines the number of concurrent identical ER that are submitted to the
	// verifier node
	const ConcurrencyDegree = 10

	// mocks an execution ID and a verifier engine
	// also mocks the reception of 10 concurrent identical execution results
	// as well as a random execution receipt (er) and its mocked execution receipt
	exeID, vrfy, er := v.ConcurrencyTestSetup(ConcurrencyDegree, 100)

	// emitting an execution receipt form the execution node
	errCount := 0
	for i := 0; i < ConcurrencyDegree; i++ {
		err := vrfy.Process(exeID.NodeID, er)
		if err != nil {
			errCount++
		}
	}
	// all ERs are the same, so only one of them should be processed
	assert.Equal(v.T(), errCount, ConcurrencyDegree-1)

	vrfy.wg.Wait()
	v.con.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
	v.state.AssertExpectations(v.T())
}

// TestProcessHappyPathConcurrentERs covers the happy path of the verifier engine on concurrently
// receiving a valid execution receipt several times each over a different threads
// In other words, this test concerns invoking the Process method over threads
// The expected behavior is to verify only a single copy of those receipts while dismissing the rest
func (v *VerifierEngineTestSuit) TestProcessHappyPathConcurrentERsConcurrently() {
	// Todo this test is currently broken as it assumes the Process method of engine to
	// be called sequentially and not over a thread
	// We skip it as it is not required for MVP
	// Skipping this test for now
	v.T().SkipNow()

	// ConcurrencyDegree defines the number of concurrent identical ER that are submitted to the
	// verifier node
	const ConcurrencyDegree = 10

	// mocks an execution ID and a verifier engine
	// also mocks the reception of 10 concurrent identical execution results
	// as well as a random execution receipt (er) and its mocked execution receipt
	exeID, vrfy, er := v.ConcurrencyTestSetup(ConcurrencyDegree, 100)

	// emitting an execution receipt form the execution node
	errCount := 0
	mu := sync.Mutex{}
	for i := 0; i < ConcurrencyDegree; i++ {
		go func() {
			err := vrfy.Process(exeID.NodeID, er)
			if err != nil {
				mu.Lock()
				errCount++
				mu.Unlock()
			}
		}()
	}
	// all ERs are the same, so only one of them should be processed
	assert.Equal(v.T(), errCount, ConcurrencyDegree-1)

	vrfy.wg.Wait()
	v.con.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
	v.state.AssertExpectations(v.T())
}

// TestProcessHappyPathConcurrentDifferentERs covers the happy path of the verifier engine on concurrently
// receiving several valid execution receipts.
// The expected behavior is to verify all of them and emit one submission of result approval per input receipt
func (v *VerifierEngineTestSuit) TestProcessHappyPathConcurrentDifferentERs() {
	// ConcurrencyDegree defines the number of concurrent identical ER that are submitted to the
	// verifier node
	const ConcurrencyDegree = 10

	// creating a new engine
	vrfy := v.TestNewEngine()

	// a mock staked execution node for generating a mock execution receipt
	exeID := flow.Identity{
		NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
		Address: "mock-en-address",
		Role:    flow.RoleExecution,
	}

	// mocking state for Final().Identity(originID) in onExecutionReceipt method
	v.state.On("Final").Return(v.ss).Times(ConcurrencyDegree)
	v.ss.On("Identity", exeID.NodeID).Return(exeID, nil).Times(ConcurrencyDegree)

	// generating a set of mock consensus ids
	consIDs := generateMockConsensusIDs(100)

	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus)) call in verify method
	// since ERs distinct, distinct calls for retrieving consensus nodes identity should happen
	v.state.On("Final").Return(v.ss).Times(ConcurrencyDegree)
	v.ss.On("Identities", testifymock.Anything).Return(consIDs, nil).Times(ConcurrencyDegree)

	testTable := [ConcurrencyDegree]struct {
		receipt *flow.ExecutionReceipt
		params  []interface{} // parameters of the resulted Submit method of the engine corresponding to receipt
	}{}

	// preparing the test table
	for i := 0; i < ConcurrencyDegree; i++ {
		// generating a random execution receipt and its corresponding result approval
		er := verification.RandomERGen()
		restApprov := verification.RandomRAGen(er)
		params := genSubmitParams(restApprov, consIDs)
		testTable[i].receipt = er
		testTable[i].params = params
	}

	// emitting an execution receipt form the execution node
	errCount := 0
	for i := 0; i < ConcurrencyDegree; i++ {
		// the happy path ends by the verifier engine emitting a
		// result approval to ONLY all the consensus nodes
		// since ERs distinct, distinct calls for submission should happen
		v.con.On("Submit", testTable[i].params...).
			Return(nil).
			Once()

		err := vrfy.Process(exeID.NodeID, testTable[i].receipt)
		if err != nil {
			errCount++
		}
	}
	// all ERs are the same, so only one of them should be processed
	assert.Equal(v.T(), errCount, 0)

	vrfy.wg.Wait()
	v.con.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
	v.state.AssertExpectations(v.T())
}

// TestProcessHappyPath covers the happy path of the verifier engine on receiving a valid execution receipt
// The expected behavior is to verify the receipt and emit a result approval to all consensus nodes
func (v *VerifierEngineTestSuit) TestProcessHappyPath() {
	// creating a new engine
	vrfy := v.TestNewEngine()

	// a mock staked execution node for generating a mock execution receipt
	exeID := flow.Identity{
		NodeID:  flow.Identifier{0x02, 0x02, 0x02, 0x02},
		Address: "mock-en-address",
		Role:    flow.RoleExecution,
	}

	// mocking state fo Final().Identity(originID) call in onExecutionReceipt method
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identity", exeID.NodeID).Return(exeID, nil).Once()

	// generating a set of mock consensus ids
	consIDs := generateMockConsensusIDs(100)

	// mocking for Final().Identities(identity.HasRole(flow.RoleConsensus)) in verify method
	v.state.On("Final").Return(v.ss).Once()
	v.ss.On("Identities", testifymock.Anything).Return(consIDs, nil)

	// generating a random execution receipt and its corresponding result approval
	er := verification.RandomERGen()
	restApprov := verification.RandomRAGen(er)

	// the happy path ends by the verifier engine emitting a
	// result approval to ONLY all the consensus nodes
	// generating and mocking parameters of Submit method
	params := genSubmitParams(restApprov, consIDs)
	v.con.On("Submit", params...).
		Return(nil).
		Once()

	// emitting an execution receipt form the execution node
	err := vrfy.Process(exeID.NodeID, er)
	assert.Nil(v.T(), err, "failed processing execution receipt")

	vrfy.wg.Wait()
	v.con.AssertExpectations(v.T())
	v.ss.AssertExpectations(v.T())
	v.state.AssertExpectations(v.T())
}

// generateMockIdentities generates and returns set of random nodes with different roles
// the distribution of the roles is uniforms but not guaranteed
// size: total number of nodes
func generateMockIdentities(size int) flow.IdentityList {
	var identities flow.IdentityList
	for i := 0; i < size; i++ {
		// creating mock identities as a random byte array
		var nodeID flow.Identifier
		rand.Read(nodeID[:])
		address := fmt.Sprintf("address%d", i)
		var role flow.Role
		switch rand.Intn(5) {
		case 0:
			role = flow.RoleCollection
		case 1:
			role = flow.RoleConsensus
		case 2:
			role = flow.RoleExecution
		case 3:
			role = flow.RoleVerification
		case 4:
			role = flow.RoleObservation
		}
		id := flow.Identity{
			NodeID:  nodeID,
			Address: address,
			Role:    role,
		}
		identities = append(identities, id)
	}
	return identities
}

// generateMockConsensusIDs generates and returns set of random consensus nodes
// size: total number of nodes
func generateMockConsensusIDs(size int) flow.IdentityList {
	var identities flow.IdentityList
	for i := 0; i < size; i++ {
		// creating mock identities as a random byte array
		var nodeID flow.Identifier
		rand.Read(nodeID[:])
		address := fmt.Sprintf("address%d", i)
		id := flow.Identity{
			NodeID:  nodeID,
			Address: address,
			Role:    flow.RoleConsensus,
		}
		identities = append(identities, id)
	}
	return identities
}

// newMockVrfyID returns a new mocked identity for verification node
func newMockVrfyID() flow.Identity {
	return flow.Identity{
		NodeID:  flow.Identifier{0x01, 0x01, 0x01, 0x01},
		Address: "mock-vn-address",
		Role:    flow.RoleVerification,
	}
}

// genSubmitParams generates the parameters of network.Conduit.Submit method for emitting the
// result approval. On receiving a result approval and identifiers of consensus nodes, it returns
// a slice with the result approval as the first element followed by the ids of consensus nodes.
func genSubmitParams(ra *flow.ResultApproval, consIDs flow.IdentityList) []interface{} {
	// extracting mock consensus nodes IDs
	params := []interface{}{ra}
	for _, targetID := range consIDs.NodeIDs() {
		params = append(params, targetID)
	}
	return params
}

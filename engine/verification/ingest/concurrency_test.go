package ingest_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	testifymock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/crypto/random"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/testutil"
	"github.com/dapperlabs/flow-go/engine/testutil/mock"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/engine/verification/test"
	chmodel "github.com/dapperlabs/flow-go/model/chunks"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	network "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// testConcurrency evaluates behavior of verification node against receiving concurrent receipts from
// different sources
func TestConcurrency(t *testing.T) {
	testcases := []struct {
		erCount, // number of execution receipts
		senderCount, // number of (concurrent) senders for each execution receipt
		chunksNum int // number of chunks in each execution receipt
	}{
		{
			erCount:     1,
			senderCount: 1,
			chunksNum:   1,
		}, {
			erCount:     1,
			senderCount: 10,
			chunksNum:   1,
		},
		{
			erCount:     10,
			senderCount: 1,
			chunksNum:   1,
		},
		{
			erCount:     5,
			senderCount: 10,
			chunksNum:   1,
		},
		// multiple chunks receipts
		{
			erCount:     1,
			senderCount: 1,
			chunksNum:   5, // choosing a higher number makes the test longer and longer timeout needed
		},
		{
			erCount:     1,
			senderCount: 10,
			chunksNum:   10,
		},
		{
			erCount:     3,
			senderCount: 1,
			chunksNum:   5,
		},
		{
			erCount:     3,
			senderCount: 5,
			chunksNum:   2, // choosing a higher number makes the test longer and longer timeout needed
		},
	}

	for _, tc := range testcases {
		t.Run(fmt.Sprintf("%d-ers/%d-senders/%d-chunks", tc.erCount, tc.senderCount, tc.chunksNum), func(t *testing.T) {
			testConcurrency(t, tc.erCount, tc.senderCount, tc.chunksNum)
		})
	}
}

func testConcurrency(t *testing.T, erCount, senderCount, chunksNum int) {
	hub := stub.NewNetworkHub()

	// creates test id for each role
	colID := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))
	conID := unittest.IdentityFixture(unittest.WithRole(flow.RoleConsensus))
	exeID := unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution))
	verID := unittest.IdentityFixture(unittest.WithRole(flow.RoleVerification))

	identities := flow.IdentityList{colID, conID, exeID, verID}

	// new chunk assignment
	assignment := chmodel.NewAssignment()

	// create `erCount` ER fixtures that will be concurrently delivered
	ers := make([]verification.CompleteExecutionResult, 0)
	// list of assigned chunks to the verifier node
	vChunks := make([]*verification.VerifiableChunk, 0)
	// a counter to assign chunks every other one, so to check if
	// ingest only sends the assigned chunks to verifier

	for i := 0; i < erCount; i++ {
		er := test.CompleteExecutionResultFixture(t, chunksNum)
		ers = append(ers, er)
		// assigns all chunks to the verifier node
		for j, chunk := range er.Receipt.ExecutionResult.Chunks {
			assignment.Add(chunk, []flow.Identifier{verID.NodeID})

			var endState flow.StateCommitment
			// last chunk
			if j == len(er.Receipt.ExecutionResult.Chunks)-1 {
				endState = er.Receipt.ExecutionResult.FinalStateCommit
			} else {
				endState = er.Receipt.ExecutionResult.Chunks[j+1].StartState
			}

			vc := &verification.VerifiableChunk{
				ChunkIndex: chunk.Index,
				EndState:   endState,
				Block:      er.Block,
				Receipt:    er.Receipt,
				Collection: er.Collections[chunk.Index],
			}
			vChunks = append(vChunks, vc)
		}
	}

	// set up mock verifier engine that asserts each receipt is submitted
	// to the verifier exactly once.
	verifierEng, verifierEngWG := setupMockVerifierEng(t, vChunks)
	assigner := NewMockAssigner(verID.NodeID)
	verNode := testutil.VerificationNode(t, hub, verID, identities, assigner,
		testutil.WithVerifierEngine(verifierEng))

	colNode := testutil.CollectionNode(t, hub, colID, identities)

	// mock the execution node with a generic node and mocked engine
	// to handle requests for chunk state
	exeNode := testutil.GenericNode(t, hub, exeID, identities)
	setupMockExeNode(t, exeNode, verID.NodeID, ers)

	verNet, ok := hub.GetNetwork(verID.NodeID)
	assert.True(t, ok)

	// the wait group tracks goroutines for each ER sending it to VER
	var senderWG sync.WaitGroup
	senderWG.Add(erCount * senderCount)

	var blockStorageLock sync.Mutex

	for _, completeER := range ers {
		for _, coll := range completeER.Collections {
			err := colNode.Collections.Store(coll)
			assert.Nil(t, err)
		}

		// spin up `senderCount` sender goroutines to mimic receiving
		// the same resource multiple times
		for i := 0; i < senderCount; i++ {
			go func(j int, id flow.Identifier, block *flow.Block, receipt *flow.ExecutionReceipt) {

				sendBlock := func() {
					// adds the block to the storage of the node
					// Note: this is done by the follower
					// this block should be done in a thread-safe way
					blockStorageLock.Lock()
					// we don't check for error as it definitely returns error when we
					// have duplicate blocks, however, this is not the concern for this test
					_ = verNode.BlockStorage.Store(block)
					blockStorageLock.Unlock()

					// casts block into a Hotstuff block for notifier
					hotstuffBlock := &model.Block{
						BlockID:     block.ID(),
						View:        block.View,
						ProposerID:  block.ProposerID,
						QC:          nil,
						PayloadHash: block.Hash(),
						Timestamp:   block.Timestamp,
					}
					verNode.IngestEngine.OnFinalizedBlock(hotstuffBlock)
				}

				sendReceipt := func() {
					err := verNode.IngestEngine.Process(exeID.NodeID, receipt)
					require.NoError(t, err)
				}

				switch j % 2 {
				case 0:
					// block then receipt
					sendBlock()
					verNet.DeliverAll(true)
					// allow another goroutine to run before sending receipt
					time.Sleep(time.Nanosecond)
					sendReceipt()
				case 1:
					// receipt then block
					sendReceipt()
					verNet.DeliverAll(true)
					// allow another goroutine to run before sending block
					time.Sleep(time.Nanosecond)
					sendBlock()
				}

				verNet.DeliverAll(true)
				go senderWG.Done()
			}(i, completeER.Receipt.ExecutionResult.ID(), completeER.Block, completeER.Receipt)
		}
	}

	// wait for all ERs to be sent to VER
	unittest.AssertReturnsBefore(t, senderWG.Wait, 3*time.Second)
	verNet.DeliverAll(false)
	unittest.AssertReturnsBefore(t, verifierEngWG.Wait, 3*time.Second)
	verNet.DeliverAll(false)

	exeNode.Done()
	colNode.Done()
	verNode.Done()
}

// setupMockExeNode sets up a mocked execution node that responds to requests for
// chunk states. Any requests that don't correspond to an execution receipt in
// the input ers list result in the test failing.
func setupMockExeNode(t *testing.T, node mock.GenericNode, verID flow.Identifier, ers []verification.CompleteExecutionResult) {
	eng := new(network.Engine)
	chunksConduit, err := node.Net.Register(engine.ChunkDataPackProvider, eng)
	assert.Nil(t, err)

	reqChunksExe := make(map[flow.Identifier]struct{})

	eng.On("Process", verID, testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			if req, ok := args[1].(*messages.ChunkDataPackRequest); ok {
				if _, ok := reqChunksExe[req.ChunkID]; ok {
					// duplicate request detected
					t.Fail()
				}
				reqChunksExe[req.ChunkID] = struct{}{}
				for _, er := range ers {
					for _, chunk := range er.Receipt.ExecutionResult.Chunks {
						if chunk.ID() == req.ChunkID {
							res := &messages.ChunkDataPackResponse{
								Data: *er.ChunkDataPacks[chunk.Index],
							}
							err := chunksConduit.Submit(res, verID)
							assert.Nil(t, err)
							return
						}
					}
				}
			}
			t.Logf("invalid chunk request (%T): %v ", args[1], args[1])
			t.Fail()
		}).
		Return(nil)

}

// setupMockVerifierEng sets up a mock verifier engine that asserts that a set
// of chunks are delivered to it exactly once each.
// Returns the mock engine and a wait group that unblocks when all ERs are received.
func setupMockVerifierEng(t *testing.T, vChunks []*verification.VerifiableChunk) (*network.Engine, *sync.WaitGroup) {
	eng := new(network.Engine)

	// keep track of which verifiable chunks we have received
	receivedChunks := make(map[flow.Identifier]struct{})
	var (
		// decrement the wait group when each verifiable chunk received
		wg sync.WaitGroup
		// check one verifiable chunk at a time to ensure dupe checking works
		mu sync.Mutex
	)
	wg.Add(len(vChunks))

	eng.On("ProcessLocal", testifymock.Anything).
		Run(func(args testifymock.Arguments) {
			mu.Lock()
			defer mu.Unlock()

			vc, ok := args[0].(*verification.VerifiableChunk)
			assert.True(t, ok)

			chunk, ok := vc.Receipt.ExecutionResult.Chunks.ByIndex(vc.ChunkIndex)
			require.True(t, ok, "chunk out of range requested")
			vID := chunk.ID()
			// ensure there are no dupe chunks
			_, alreadySeen := receivedChunks[vID]
			if alreadySeen {
				t.Logf("received duplicated chunk (id=%s)", vID)
				t.Fail()
				return
			}

			// ensure the received chunk matches one we expect
			for _, vc := range vChunks {
				chunk, ok := vc.Receipt.ExecutionResult.Chunks.ByIndex(vc.ChunkIndex)
				require.True(t, ok, "chunk out of range requested")
				if chunk.ID() == vID {
					// mark it as seen and decrement the waitgroup
					receivedChunks[vID] = struct{}{}
					wg.Done()
					return
				}
			}

			// the received chunk doesn't match any expected ERs
			t.Logf("received unexpected ER (id=%s)", vID)
			t.Fail()
		}).
		Return(nil)

	return eng, &wg
}

type MockAssigner struct {
	me flow.Identifier
}

func NewMockAssigner(id flow.Identifier) *MockAssigner {
	return &MockAssigner{me: id}
}

// Assign assigns all input chunks to the verifer node
func (m *MockAssigner) Assign(ids flow.IdentityList, chunks flow.ChunkList, rng random.Rand) (*chmodel.Assignment, error) {
	if len(chunks) == 0 {
		return nil, fmt.Errorf("assigner called with empty chunk list")
	}
	a := chmodel.NewAssignment()
	for _, c := range chunks {
		a.Add(c, flow.IdentifierList{m.me})
	}

	return a, nil
}

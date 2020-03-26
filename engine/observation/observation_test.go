package observation

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/engine/observation/ingestion"
	obs "github.com/dapperlabs/flow-go/engine/observation/mock"
	"github.com/dapperlabs/flow-go/engine/observation/rpc"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	mockmodule "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/protobuf/sdk/entities"

	networkmock "github.com/dapperlabs/flow-go/network/mock"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	bstorage "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite
	state              *protocol.State
	snapshot           *protocol.Snapshot
	log                zerolog.Logger
	net                *mockmodule.Network
	collClient         *obs.ObserveServiceClient
	execClient         *obs.ObserveServiceClient
	collectionsConduit *networkmock.Conduit
}

// TestObservation tests scenarios which exercise multiple API calls using both the RPC handler and the ingest engine
// and using a real badger storage.
func TestObservation(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	suite.log = zerolog.Logger{}
	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	suite.state.On("Final").Return(suite.snapshot, nil).Maybe()
	suite.collClient = new(obs.ObserveServiceClient)
	suite.execClient = new(obs.ObserveServiceClient)
	suite.net = new(mockmodule.Network)
	suite.collectionsConduit = &networkmock.Conduit{}
}

func (suite *Suite) TestSendAndGetTransaction() {

	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		transaction := unittest.TransactionFixture(func(t *flow.Transaction) {
			t.Nonce = 0
			t.ComputeLimit = 0
		})

		// create storage
		collections := bstorage.NewCollections(db)
		transactions := bstorage.NewTransactions(db)
		handler := rpc.NewHandler(suite.log, suite.state, nil, suite.collClient, nil, nil, collections, transactions)

		expected := convert.TransactionToMessage(transaction.TransactionBody)
		sendReq := &observation.SendTransactionRequest{
			Transaction: expected,
		}
		sendResp := observation.SendTransactionResponse{}
		suite.collClient.On("SendTransaction", mock.Anything, mock.Anything).Return(&sendResp, nil).Once()

		// Send transaction
		resp, err := handler.SendTransaction(context.Background(), sendReq)
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), resp)

		id := transaction.ID()
		getReq := &observation.GetTransactionRequest{
			Id: id[:],
		}

		// Get transaction
		gResp, err := handler.GetTransaction(context.Background(), getReq)
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), gResp)

		actual := gResp.Transaction

		require.Equal(suite.T(), expected, actual)
	})
}

func (suite *Suite) TestGetBlockByIDAndHeight() {

	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		// test block1 get by ID
		block1 := unittest.BlockHeaderFixture()
		// test block2 get by height
		block2 := unittest.BlockHeaderFixture()
		block2.Height = 2

		blocks := bstorage.NewHeaders(db)
		require.NoError(suite.T(), blocks.Store(&block1))
		require.NoError(suite.T(), blocks.Store(&block2))

		// the follower logic should update height index on the block storage when a block is finalized
		err := db.Update(operation.InsertNumber(block2.Height, block2.ID()))
		require.NoError(suite.T(), err)

		handler := rpc.NewHandler(suite.log, suite.state, nil, suite.collClient, nil, blocks, nil, nil)

		// get by ID
		id := block1.ID()
		req1 := &observation.GetBlockHeaderByIDRequest{
			Id: id[:],
		}

		resp, err := handler.GetBlockHeaderByID(context.Background(), req1)

		// assert it is indeed block1
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), resp)
		actual := resp.Block
		expected, _ := convert.BlockHeaderToMessage(&block1)
		require.Equal(suite.T(), expected, *actual)

		// get by height
		req2 := &observation.GetBlockHeaderByHeightRequest{
			Height: block2.Height,
		}

		// assert it is indeed block2
		resp, err = handler.GetBlockHeaderByHeight(context.Background(), req2)
		require.NoError(suite.T(), err)
		require.NotNil(suite.T(), resp)
		actual = resp.Block
		expected, _ = convert.BlockHeaderToMessage(&block2)
		require.Equal(suite.T(), expected, *actual)
	})
}

// TestGetSealedTransaction tests that transactions status of transaction that belongs to a sealed blocked
// is reported as sealed
func (suite *Suite) TestGetSealedTransaction() {
	unittest.RunWithBadgerDB(suite.T(), func(db *badger.DB) {
		// create block -> collection -> transactions and seal
		block, collection, seal := suite.createChain()

		// setup mocks
		originID := unittest.IdentifierFixture()
		suite.net.On("Register", uint8(engine.CollectionProvider), mock.Anything).
			Return(suite.collectionsConduit, nil).
			Once()
		colIdentities := unittest.IdentityListFixture(1, unittest.WithRole(flow.RoleCollection))
		suite.snapshot.On("Identities", mock.Anything).Return(colIdentities, nil).Once()
		suite.collectionsConduit.On("Submit", mock.Anything, mock.Anything).Return(nil).Times(len(block.Guarantees))

		// initialize storage
		blocks := bstorage.NewBlocks(db)
		headers := bstorage.NewHeaders(db)
		collections := bstorage.NewCollections(db)
		transactions := bstorage.NewTransactions(db)

		// create the ingest engine
		ingestEng, err := ingestion.New(suite.log, suite.net, suite.state, nil, nil, blocks, headers, collections, transactions)
		require.NoError(suite.T(), err)

		// create the handler (called by the grpc engine)
		handler := rpc.NewHandler(suite.log, suite.state, nil, suite.collClient, blocks, headers, collections, transactions)

		// 1. Assume that follower engine updated the block storage and the protocol state. The block is reported as sealed
		err = blocks.Store(&block)
		require.NoError(suite.T(), err)
		suite.snapshot.On("Seal").Return(seal, nil).Once()

		// 2. Ingest engine was notified by the follower engine with a new block
		err = ingestEng.Process(originID, &block)
		require.NoError(suite.T(), err)

		// 3. Ingest engine requests all collections of the block
		suite.collectionsConduit.AssertExpectations(suite.T())

		// 4. Ingest engine receives the requested collection
		cr := &messages.CollectionResponse{Collection: collection}
		err = ingestEng.Process(originID, cr)
		require.NoError(suite.T(), err)

		// 5. client requests a transaction
		tx := collection.Transactions[0]
		id := tx.ID()
		getReq := &observation.GetTransactionRequest{
			Id: id[:],
		}
		gResp, err := handler.GetTransaction(context.Background(), getReq)
		require.NoError(suite.T(), err)
		// assert that the transaction is reported as Sealed
		require.Equal(suite.T(), entities.TransactionStatus_STATUS_SEALED, gResp.Transaction.Status)
	})
}

func (suite *Suite) createChain() (flow.Block, flow.Collection, flow.Seal) {
	collection := unittest.CollectionFixture(10)
	cg := &flow.CollectionGuarantee{
		CollectionID: collection.ID(),
		Signatures:   []crypto.Signature{[]byte("signature A")},
	}
	block := unittest.BlockFixture()
	block.Guarantees = []*flow.CollectionGuarantee{cg}

	seal := flow.Seal{
		BlockID: block.ID(),
	}

	return block, collection, seal
}

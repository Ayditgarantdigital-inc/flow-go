package collection

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/collection/ingest"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	clusterstate "github.com/dapperlabs/flow-go/state/cluster/badger"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const defaultTimeout = 10 * time.Second

// default set of non-collection nodes
func defaultOtherNodes() []testnet.NodeConfig {
	var (
		conNode1 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel("info"))
		conNode2 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel("info"))
		conNode3 = testnet.NewNodeConfig(flow.RoleConsensus, testnet.WithLogLevel("info"))
		exeNode  = testnet.NewNodeConfig(flow.RoleExecution, testnet.WithLogLevel("info"))
		verNode  = testnet.NewNodeConfig(flow.RoleVerification, testnet.WithLogLevel("info"))
	)

	return []testnet.NodeConfig{conNode1, conNode2, conNode3, exeNode, verNode}
}

// Tests sending various invalid transactions to a single-cluster configuration
// and ensures that they are rejected by the collection node and not included in
// any collection.
func TestTransactionIngress_InvalidTransaction(t *testing.T) {
	var (
		colNode1 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(1))
		colNode2 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(2))
		colNode3 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(3))
	)

	nodes := append([]testnet.NodeConfig{colNode1, colNode2, colNode3}, defaultOtherNodes()...)
	conf := testnet.NetworkConfig{Nodes: nodes}

	net, err := testnet.PrepareFlowNetwork(t, "col_invalid_txns", conf)
	require.Nil(t, err)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Cleanup()

	// we will test against COL1
	colContainer1, ok := net.ContainerByID(colNode1.Identifier)
	assert.True(t, ok)

	port, ok := colContainer1.Ports[testnet.ColNodeAPIPort]
	assert.True(t, ok)

	client, err := testnet.NewClient(fmt.Sprintf(":%s", port))
	assert.Nil(t, err)

	t.Run("missing reference block hash", func(t *testing.T) {
		txDSL := unittest.TransactionDSLFixture()
		malformed := unittest.TransactionBodyFixture(unittest.WithTransactionDSL(txDSL))
		malformed.ReferenceBlockID = flow.ZeroID

		expected := ingest.ErrIncompleteTransaction{Missing: malformed.MissingFields()}

		ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
		err := client.SignAndSendTransaction(ctx, malformed)
		unittest.AssertErrSubstringMatch(t, expected, err)
	})

	t.Run("missing script", func(t *testing.T) {
		malformed := unittest.TransactionBodyFixture()
		malformed.Script = nil

		expected := ingest.ErrIncompleteTransaction{Missing: malformed.MissingFields()}

		ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
		err := client.SignAndSendTransaction(ctx, malformed)
		unittest.AssertErrSubstringMatch(t, expected, err)
	})

	t.Run("unparseable script", func(t *testing.T) {
		// TODO script parsing not implemented
		t.Skip()
	})
	t.Run("invalid signature", func(t *testing.T) {
		// TODO signature validation not implemented
		t.Skip()
	})
	t.Run("invalid nonce", func(t *testing.T) {
		// TODO nonce validation not implemented
		t.Skip()
	})
	t.Run("insufficient payer balance", func(t *testing.T) {
		// TODO balance checking not implemented
		t.Skip()
	})
	t.Run("expired transaction", func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		t.Skip()
	})
	t.Run("non-existent reference block ID", func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		t.Skip()
	})
}

func TestTransactionIngress_ValidTransaction(t *testing.T) {

	var (
		colNode1 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(1))
		colNode2 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(2))
		colNode3 = testnet.NewNodeConfig(flow.RoleCollection, testnet.WithIDInt(3))
	)

	nodes := append([]testnet.NodeConfig{colNode1, colNode2, colNode3}, defaultOtherNodes()...)
	conf := testnet.NetworkConfig{Nodes: nodes}

	net, err := testnet.PrepareFlowNetwork(t, "col_valid_txns", conf)
	require.Nil(t, err)

	ctx := context.Background()

	net.Start(ctx)
	defer net.Cleanup()

	// we will test against COL1
	colContainer1, ok := net.ContainerByID(colNode1.Identifier)
	assert.True(t, ok)

	port, ok := colContainer1.Ports[testnet.ColNodeAPIPort]
	assert.True(t, ok)

	client, err := testnet.NewClient(fmt.Sprintf(":%s", port))

	t.Run("valid transaction", func(t *testing.T) {
		tx := unittest.TransactionBodyFixture()
		tx, err := client.SignTransaction(tx)
		assert.Nil(t, err)
		t.Log("sending transaction: ", tx.ID())

		ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
		defer cancel()
		err = client.SendTransaction(ctx, tx)
		assert.Nil(t, err)

		// wait for consensus to complete
		//TODO we should listen for collection guarantees instead, but this is blocked
		// ref: https://github.com/dapperlabs/flow-go/issues/3021
		time.Sleep(10 * time.Second)

		// TODO stop then start containers
		err = net.StopContainers()
		assert.Nil(t, err)

		identities := net.Identities()

		chainID := protocol.ChainIDForCluster(identities.Filter(filter.HasRole(flow.RoleCollection)))

		// get database for COL1
		db, err := colContainer1.DB()
		require.Nil(t, err)

		state, err := clusterstate.NewState(db, chainID)
		assert.Nil(t, err)

		// the transaction should be included in exactly one collection
		head, err := state.Final().Head()
		assert.Nil(t, err)

		foundTx := false
		for head.Height > 0 {
			collection, err := state.AtBlockID(head.ID()).Collection()
			assert.Nil(t, err)

			head, err = state.AtBlockID(head.ParentID).Head()
			assert.Nil(t, err)

			if collection.Len() == 0 {
				continue
			}

			for _, txID := range collection.Transactions {
				assert.Equal(t, tx.ID(), txID, "found unexpected transaction")
				if txID == tx.ID() {
					assert.False(t, foundTx, "found duplicate transaction")
					foundTx = true
				}
			}
		}

		assert.True(t, foundTx)
	})
}

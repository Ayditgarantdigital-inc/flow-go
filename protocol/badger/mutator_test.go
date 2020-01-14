// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"math/rand"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func init() {
	rand.Seed(time.Now().UnixNano())
}

func TestBootStrapValid(t *testing.T) {

	ids := flow.IdentityList{
		{NodeID: flow.Identifier{0x01}, Address: "a1", Role: flow.Role(1), Stake: 1},
		{NodeID: flow.Identifier{0x02}, Address: "a2", Role: flow.Role(2), Stake: 2},
		{NodeID: flow.Identifier{0x03}, Address: "a3", Role: flow.Role(3), Stake: 3},
	}

	header := flow.Header{
		Number:      0,
		Timestamp:   time.Now().UTC(),
		ParentID:    flow.ZeroID,
		PayloadHash: flow.Identifier{0x11},
		Signatures:  []crypto.Signature{[]byte("signature")},
	}

	content := flow.Content{
		Identities: ids,
		Guarantees: nil,
	}

	genesis := flow.Block{
		Header:  header,
		Payload: content.Payload(),
		Content: content,
	}

	blockID := genesis.ID()

	unittest.RunWithBadgerDB(t, func(db *badger.DB) {

		mutator := &Mutator{state: &State{db: db}}
		err := mutator.Bootstrap(&genesis)
		require.Nil(t, err)

		var boundary uint64
		err = db.View(operation.RetrieveBoundary(&boundary))
		require.Nil(t, err)

		var storedID flow.Identifier
		err = db.View(operation.RetrieveBlockID(0, &storedID))
		require.Nil(t, err)

		var storedHeader flow.Header
		err = db.View(operation.RetrieveHeader(genesis.ID(), &storedHeader))
		require.Nil(t, err)

		var storedIDs flow.IdentityList
		err = db.View(operation.RetrieveIdentities(genesis.ID(), &storedIDs))
		require.Nil(t, err)

		assert.Zero(t, boundary)
		assert.Equal(t, blockID, storedID)
		assert.Equal(t, header, storedHeader)
		assert.Equal(t, ids, storedIDs)

		for _, id := range ids {

			var role flow.Role
			err = db.View(operation.RetrieveRole(id.NodeID, &role))
			require.Nil(t, err)

			var address string
			err = db.View(operation.RetrieveAddress(id.NodeID, &address))
			require.Nil(t, err)

			var delta int64
			err = db.View(operation.RetrieveDelta(header.Number, id.Role, id.NodeID, &delta))
			require.Nil(t, err)

			assert.Equal(t, id.Role, role)
			assert.Equal(t, id.Address, address)
			assert.Equal(t, int64(id.Stake), delta)
		}
	})
}

func TestBootstrapDuplicateID(t *testing.T) {
	// TODO
}

func TestBootstrapZeroStake(t *testing.T) {
	// TODO
}

func TestBootstrapExistingRole(t *testing.T) {
	// TODO
}

func TestBootstrapExistingAddress(t *testing.T) {
	// TODO
}

func TestBootstrapNonZeroNumber(t *testing.T) {
	// TODO
}

func TestBootstrapNonZeroParent(t *testing.T) {
	// TODO
}

func TestBootstrapNonEmptyCollections(t *testing.T) {
	// TODO
}

func TestExtendValid(t *testing.T) {
	// TODO
}

func TestExtendDuplicateID(t *testing.T) {
	// TODO
}

func TestExtendZeroStake(t *testing.T) {
	// TODO
}

func TestExtendExistingRole(t *testing.T) {
	// TODO
}

func TestExtendExistingAddress(t *testing.T) {
	// TODO
}

func TestExtendMissingParent(t *testing.T) {
	// TODO
}

func TestExtendNumberTooSmall(t *testing.T) {
	// TODO
}

func TestExtendNotConnected(t *testing.T) {
	// TODO
}

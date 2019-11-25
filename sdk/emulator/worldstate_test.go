package emulator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

// addTwoScript runs a script that adds 2 to a value.
const addTwoScript = `
	pub fun main(account: Account) {
		account.storage[Int] = (account.storage[Int] ?? 0) + 2
	}
`

const sampleCall = `
	pub fun main(): Int {
		return getAccount("%s").storage[Int] ?? 0
	}
`

func TestWorldStates(t *testing.T) {
	b := NewEmulatedBlockchain(DefaultOptions)

	accountAddress := b.RootAccountAddress()

	// Create 3 signed transactions (tx1, tx2, tx3)
	tx1 := flow.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       accountAddress,
		ScriptAccounts:     []flow.Address{accountAddress},
	}

	sig, err := keys.SignTransaction(tx1, b.RootKey())
	assert.Nil(t, err)

	tx1.AddSignature(accountAddress, sig)

	tx2 := flow.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              2,
		ComputeLimit:       10,
		PayerAccount:       accountAddress,
		ScriptAccounts:     []flow.Address{accountAddress},
	}

	sig, err = keys.SignTransaction(tx2, b.RootKey())
	assert.Nil(t, err)

	tx2.AddSignature(accountAddress, sig)

	tx3 := flow.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              3,
		ComputeLimit:       10,
		PayerAccount:       accountAddress,
		ScriptAccounts:     []flow.Address{accountAddress},
	}

	sig, err = keys.SignTransaction(tx3, b.RootKey())
	assert.Nil(t, err)

	tx3.AddSignature(accountAddress, sig)

	ws1 := b.pendingWorldState.Hash()
	t.Logf("initial world state: %x\n", ws1)

	// Tx pool contains nothing
	assert.Len(t, b.txPool, 0)

	// Submit tx1
	err = b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	ws2 := b.pendingWorldState.Hash()
	t.Logf("world state after tx1: %x\n", ws2)

	// tx1 included in tx pool
	assert.Len(t, b.txPool, 1)
	// World state updates
	assert.NotEqual(t, ws1, ws2)

	// Submit tx1 again
	err = b.SubmitTransaction(tx1)
	assert.NotNil(t, err)

	ws3 := b.pendingWorldState.Hash()
	t.Logf("world state after dup tx1: %x\n", ws3)

	// tx1 not included in tx pool
	assert.Len(t, b.txPool, 1)
	// World state does not update
	assert.Equal(t, ws2, ws3)

	// Submit tx2
	err = b.SubmitTransaction(tx2)
	assert.Nil(t, err)

	ws4 := b.pendingWorldState.Hash()
	t.Logf("world state after tx2: %x\n", ws4)

	// tx2 included in tx pool
	assert.Len(t, b.txPool, 2)
	// World state updates
	assert.NotEqual(t, ws3, ws4)

	// Commit new block
	b.CommitBlock()
	ws5 := b.pendingWorldState.Hash()
	t.Logf("world state after commit: %x\n", ws5)

	// Tx pool cleared
	assert.Len(t, b.txPool, 0)
	// World state updates
	assert.NotEqual(t, ws4, ws5)
	// World state is indexed
	assert.Contains(t, b.worldStates, string(ws5))

	// Submit tx3
	err = b.SubmitTransaction(tx3)
	assert.Nil(t, err)

	ws6 := b.pendingWorldState.Hash()
	t.Logf("world state after tx3: %x\n", ws6)

	// tx3 included in tx pool
	assert.Len(t, b.txPool, 1)
	// World state updates
	assert.NotEqual(t, ws5, ws6)

	// Seek to committed block/world state
	b.SeekToState(ws5)
	ws7 := b.pendingWorldState.Hash()
	t.Logf("world state after seek: %x\n", ws7)

	// Tx pool cleared
	assert.Len(t, b.txPool, 0)
	// World state rollback to ws5 (before tx3)
	assert.Equal(t, ws5, ws7)
	// World state does not include tx3
	assert.False(t, b.pendingWorldState.ContainsTransaction(tx3.Hash()))

	// Seek to non-committed world state
	b.SeekToState(ws4)
	ws8 := b.pendingWorldState.Hash()
	t.Logf("world state after failed seek: %x\n", ws8)

	// World state does not rollback to ws4 (before commit block)
	assert.NotEqual(t, ws4, ws8)
}

func TestQueryByVersion(t *testing.T) {
	b := NewEmulatedBlockchain(DefaultOptions)

	accountAddress := b.RootAccountAddress()

	tx1 := flow.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              1,
		ComputeLimit:       10,
		PayerAccount:       accountAddress,
		ScriptAccounts:     []flow.Address{accountAddress},
	}

	sig, err := keys.SignTransaction(tx1, b.RootKey())
	assert.Nil(t, err)

	tx1.AddSignature(accountAddress, sig)

	tx2 := flow.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              2,
		ComputeLimit:       10,
		PayerAccount:       accountAddress,
		ScriptAccounts:     []flow.Address{accountAddress},
	}

	sig, err = keys.SignTransaction(tx2, b.RootKey())
	assert.Nil(t, err)

	tx2.AddSignature(accountAddress, sig)

	var invalidWorldState crypto.Hash

	// Submit tx1 and tx2 (logging state versions before and after)
	ws1 := b.pendingWorldState.Hash()

	err = b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	ws2 := b.pendingWorldState.Hash()

	err = b.SubmitTransaction(tx2)
	assert.Nil(t, err)

	ws3 := b.pendingWorldState.Hash()

	// Get transaction at invalid world state version (errors)
	tx, err := b.GetTransactionAtVersion(tx1.Hash(), invalidWorldState)
	assert.IsType(t, err, &ErrInvalidStateVersion{})
	assert.Nil(t, tx)

	// tx1 does not exist at ws1
	tx, err = b.GetTransactionAtVersion(tx1.Hash(), ws1)
	assert.IsType(t, err, &ErrTransactionNotFound{})
	assert.Nil(t, tx)

	// tx1 does exist at ws2
	tx, err = b.GetTransactionAtVersion(tx1.Hash(), ws2)
	assert.Nil(t, err)
	assert.NotNil(t, tx)

	// tx2 does not exist at ws2
	tx, err = b.GetTransactionAtVersion(tx2.Hash(), ws2)
	assert.IsType(t, err, &ErrTransactionNotFound{})
	assert.Nil(t, tx)

	// tx2 does exist at ws3
	tx, err = b.GetTransactionAtVersion(tx2.Hash(), ws3)
	assert.Nil(t, err)
	assert.NotNil(t, tx)

	// Call script at invalid world state version (errors)
	callScript := fmt.Sprintf(sampleCall, accountAddress)
	value, err := b.ExecuteScriptAtVersion([]byte(callScript), invalidWorldState)
	assert.IsType(t, err, &ErrInvalidStateVersion{})
	assert.Nil(t, value)

	// Value at ws1 is 0
	value, err = b.ExecuteScriptAtVersion([]byte(callScript), ws1)
	assert.Nil(t, err)
	assert.Equal(t, values.NewInt(0), value)

	// Value at ws2 is 2 (after script executed)
	value, err = b.ExecuteScriptAtVersion([]byte(callScript), ws2)
	assert.Nil(t, err)
	assert.Equal(t, values.NewInt(2), value)

	// Value at ws3 is 4 (after script executed)
	value, err = b.ExecuteScriptAtVersion([]byte(callScript), ws3)
	assert.Nil(t, err)
	assert.Equal(t, values.NewInt(4), value)

	// Pending state does not change after call scripts/get transactions
	assert.Equal(t, ws3, b.pendingWorldState.Hash())
}

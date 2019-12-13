package sctest

import (
	"crypto/rand"
	"io/ioutil"
	"testing"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/keys"
	"github.com/stretchr/testify/assert"
)

// ReadFile reads a file from the file system
func ReadFile(path string) []byte {
	contents, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return contents
}

// GetNonce returns a nonce value that is guaranteed to be unique.
var GetNonce = func() func() uint64 {
	var nonce uint64
	return func() uint64 {
		nonce++
		return nonce
	}
}()

// randomKey returns a randomly generated private key
func randomKey() flow.AccountPrivateKey {
	seed := make([]byte, 40)
	rand.Read(seed)

	privateKey, err := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256, seed)
	if err != nil {
		panic(err)
	}

	return privateKey
}

// newEmulator returns a emulator object for testing
func newEmulator() *emulator.EmulatedBlockchain {
	b, err := emulator.NewEmulatedBlockchain()
	if err != nil {
		panic(err)
	}
	return b
}

// SignAndSubmit signs a transaction with an array of signers and adds their signatures to the transaction
// Then submits the transaction to the emulator.  If the private keys don't match up with the addresses,
// the transaction will not succeed.
// shouldRevert parameter indicates whether the transaction should fail or not
// This function asserts the correct result and commits the block if it passed
func SignAndSubmit(tx flow.Transaction, b *emulator.EmulatedBlockchain, t *testing.T, signingKeys []flow.AccountPrivateKey, signingAddresses []flow.Address, shouldRevert bool) {
	// add array of signers to transaction
	for i := 0; i < len(signingAddresses); i++ {
		sig, err := keys.SignTransaction(tx, signingKeys[i])
		assert.NoError(t, err)

		tx.AddSignature(signingAddresses[i], sig)
	}

	// submit the signed transaction
	result, err := b.SubmitTransaction(tx)

	if shouldRevert {
		assert.NoError(t, err)
		assert.True(t, result.Reverted())
	} else {
		if !assert.NoError(t, err) {
			t.Log(err.Error())
		}
	}

	_, err = b.CommitBlock()
	assert.NoError(t, err)
}

// setupUsersTokens sets up two accounts with 30 Fungible Tokens each
// and a NFT collection with 1 NFT each
func setupUsersTokens(t *testing.T, b *emulator.EmulatedBlockchain, tokenAddr flow.Address, nftAddr flow.Address, signingKeys []flow.AccountPrivateKey, signingAddresses []flow.Address) {

	// add array of signers to transaction
	for i := 0; i < len(signingAddresses); i++ {
		tx := flow.Transaction{
			Script:         GenerateCreateTokenScript(tokenAddr),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{signingAddresses[i]},
		}
		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), signingKeys[i]}, []flow.Address{b.RootAccountAddress(), signingAddresses[i]}, false)

		// then deploy a NFT to the accounts
		tx = flow.Transaction{
			Script:         GenerateCreateNFTScript(nftAddr, i+1),
			Nonce:          GetNonce(),
			ComputeLimit:   20,
			PayerAccount:   b.RootAccountAddress(),
			ScriptAccounts: []flow.Address{signingAddresses[i]},
		}
		SignAndSubmit(tx, b, t, []flow.AccountPrivateKey{b.RootKey(), signingKeys[i]}, []flow.Address{b.RootAccountAddress(), signingAddresses[i]}, false)
	}
}

package emulator_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/sdk/abi/encoding"
	"github.com/dapperlabs/flow-go/sdk/abi/types"
	"github.com/dapperlabs/flow-go/sdk/abi/values"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/keys"
)

func TestSubmitTransaction(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	tx1 := flow.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	sig, err := keys.SignTransaction(tx1, b.RootKey())
	assert.Nil(t, err)

	tx1.AddSignature(b.RootAccountAddress(), sig)

	// Submit tx1
	err = b.SubmitTransaction(tx1)
	assert.Nil(t, err)

	// tx1 status becomes TransactionFinalized
	tx2, err := b.GetTransaction(tx1.Hash())
	assert.Nil(t, err)
	assert.Equal(t, flow.TransactionFinalized, tx2.Status)
}

// TODO: Add test case for missing ReferenceBlockHash
func TestSubmitInvalidTransaction(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	t.Run("EmptyTransaction", func(t *testing.T) {
		// Create empty transaction (no required fields)
		tx := flow.Transaction{}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		// Submit tx1
		err = b.SubmitTransaction(tx)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})

	t.Run("MissingScript", func(t *testing.T) {
		// Create transaction with no Script field
		tx := flow.Transaction{
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		// Submit tx1
		err = b.SubmitTransaction(tx)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})

	t.Run("MissingNonce", func(t *testing.T) {
		// Create transaction with no Nonce field
		tx := flow.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		// Submit tx1
		err = b.SubmitTransaction(tx)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})

	t.Run("MissingComputeLimit", func(t *testing.T) {
		// Create transaction with no ComputeLimit field
		tx := flow.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			PayerAccount:       b.RootAccountAddress(),
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		// Submit tx1
		err = b.SubmitTransaction(tx)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})

	t.Run("MissingPayerAccount", func(t *testing.T) {
		// Create transaction with no PayerAccount field
		tx := flow.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		// Submit tx1
		err = b.SubmitTransaction(tx)
		assert.IsType(t, err, &emulator.ErrInvalidTransaction{})
	})
}

func TestSubmitDuplicateTransaction(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	accountAddress := b.RootAccountAddress()

	tx := flow.Transaction{
		Script:             []byte(addTwoScript),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       accountAddress,
		ScriptAccounts:     []flow.Address{accountAddress},
	}

	sig, err := keys.SignTransaction(tx, b.RootKey())
	assert.Nil(t, err)

	tx.AddSignature(accountAddress, sig)

	// Submit tx1
	err = b.SubmitTransaction(tx)
	assert.Nil(t, err)

	// Submit tx1 again (errors)
	err = b.SubmitTransaction(tx)
	assert.IsType(t, err, &emulator.ErrDuplicateTransaction{})
}

func TestSubmitTransactionReverted(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	tx1 := flow.Transaction{
		Script:             []byte("invalid script"),
		ReferenceBlockHash: nil,
		Nonce:              getNonce(),
		ComputeLimit:       10,
		PayerAccount:       b.RootAccountAddress(),
		ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
	}

	sig, err := keys.SignTransaction(tx1, b.RootKey())
	assert.Nil(t, err)

	tx1.AddSignature(b.RootAccountAddress(), sig)

	// Submit invalid tx1 (errors)
	err = b.SubmitTransaction(tx1)
	assert.NotNil(t, err)

	// tx1 status becomes TransactionReverted
	tx2, err := b.GetTransaction(tx1.Hash())
	assert.Nil(t, err)
	assert.Equal(t, flow.TransactionReverted, tx2.Status)
}

func TestSubmitTransactionScriptAccounts(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	privateKeyA := b.RootKey()

	privateKeyB, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256,
		[]byte("elephant ears space cowboy octopus rodeo potato cannon pineapple"))
	publicKeyB := privateKeyB.PublicKey(keys.PublicKeyWeightThreshold)

	accountAddressA := b.RootAccountAddress()
	accountAddressB, err := b.CreateAccount([]flow.AccountPublicKey{publicKeyB}, nil, getNonce())
	assert.Nil(t, err)

	t.Run("TooManyAccountsForScript", func(t *testing.T) {
		// script only supports one account
		script := []byte("pub fun main(account: Account) {}")

		// create transaction with two accounts
		tx := flow.Transaction{
			Script:             script,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []flow.Address{accountAddressA, accountAddressB},
		}

		sigA, err := keys.SignTransaction(tx, privateKeyA)
		assert.Nil(t, err)

		sigB, err := keys.SignTransaction(tx, privateKeyB)
		assert.Nil(t, err)

		tx.AddSignature(accountAddressA, sigA)
		tx.AddSignature(accountAddressB, sigB)

		err = b.SubmitTransaction(tx)
		assert.IsType(t, &emulator.ErrTransactionReverted{}, err)
	})

	t.Run("NotEnoughAccountsForScript", func(t *testing.T) {
		// script requires two accounts
		script := []byte("pub fun main(accountA: Account, accountB: Account) {}")

		// create transaction with two accounts
		tx := flow.Transaction{
			Script:             script,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []flow.Address{accountAddressA},
		}

		sig, err := keys.SignTransaction(tx, privateKeyA)
		assert.Nil(t, err)

		tx.AddSignature(accountAddressA, sig)

		err = b.SubmitTransaction(tx)
		assert.IsType(t, &emulator.ErrTransactionReverted{}, err)
	})
}

func TestSubmitTransactionPayerSignature(t *testing.T) {
	t.Run("MissingPayerSignature", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		addressA := flow.HexToAddress("0000000000000000000000000000000000000002")

		tx := flow.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       addressA,
			ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		err = b.SubmitTransaction(tx)

		assert.IsType(t, err, &emulator.ErrMissingSignature{})
	})

	t.Run("InvalidAccount", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		invalidAddress := flow.HexToAddress("0000000000000000000000000000000000000002")

		tx := flow.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
			ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(invalidAddress, sig)

		err = b.SubmitTransaction(tx)

		assert.IsType(t, err, &emulator.ErrInvalidSignatureAccount{})
	})

	t.Run("InvalidKeyPair", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		// use key-pair that does not exist on root account
		invalidKey, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256,
			[]byte("invalid key elephant ears space cowboy octopus rodeo potato cannon"))

		tx := flow.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
			ScriptAccounts:     []flow.Address{b.RootAccountAddress()},
		}

		sig, err := keys.SignTransaction(tx, invalidKey)
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		err = b.SubmitTransaction(tx)
		assert.IsType(t, err, &emulator.ErrInvalidSignaturePublicKey{})
	})

	t.Run("KeyWeights", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		privateKeyA, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256,
			[]byte("elephant ears space cowboy octopus rodeo potato cannon pineapple"))
		publicKeyA := privateKeyA.PublicKey(keys.PublicKeyWeightThreshold / 2)

		privateKeyB, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256,
			[]byte("space cowboy elephant ears octopus rodeo potato cannon pineapple"))
		publicKeyB := privateKeyB.PublicKey(keys.PublicKeyWeightThreshold / 2)

		accountAddressA, err := b.CreateAccount([]flow.AccountPublicKey{publicKeyA, publicKeyB}, nil, getNonce())
		assert.Nil(t, err)

		script := []byte("pub fun main(account: Account) {}")

		tx := flow.Transaction{
			Script:             script,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []flow.Address{accountAddressA},
		}

		t.Run("InsufficientKeyWeight", func(t *testing.T) {
			sig, err := keys.SignTransaction(tx, privateKeyB)
			assert.Nil(t, err)

			tx.AddSignature(accountAddressA, sig)

			err = b.SubmitTransaction(tx)
			assert.IsType(t, err, &emulator.ErrMissingSignature{})
		})

		t.Run("SufficientKeyWeight", func(t *testing.T) {
			sigA, err := keys.SignTransaction(tx, privateKeyA)
			assert.Nil(t, err)

			sigB, err := keys.SignTransaction(tx, privateKeyB)
			assert.Nil(t, err)

			tx.AddSignature(accountAddressA, sigA)
			tx.AddSignature(accountAddressA, sigB)

			err = b.SubmitTransaction(tx)
			assert.Nil(t, err)
		})
	})
}

func TestSubmitTransactionScriptSignatures(t *testing.T) {
	t.Run("MissingScriptSignature", func(t *testing.T) {
		b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

		addressA := flow.HexToAddress("0000000000000000000000000000000000000002")

		tx := flow.Transaction{
			Script:             []byte(addTwoScript),
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       b.RootAccountAddress(),
			ScriptAccounts:     []flow.Address{addressA},
		}

		sig, err := keys.SignTransaction(tx, b.RootKey())
		assert.Nil(t, err)

		tx.AddSignature(b.RootAccountAddress(), sig)

		err = b.SubmitTransaction(tx)
		assert.NotNil(t, err)
		assert.IsType(t, err, &emulator.ErrMissingSignature{})
	})

	t.Run("MultipleAccounts", func(t *testing.T) {
		loggedMessages := make([]string, 0)

		b := emulator.NewEmulatedBlockchain(emulator.EmulatedBlockchainOptions{
			OnLogMessage: func(msg string) {
				loggedMessages = append(loggedMessages, msg)
			},
		})

		privateKeyA := b.RootKey()

		privateKeyB, _ := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA3_256,
			[]byte("elephant ears space cowboy octopus rodeo potato cannon pineapple"))
		publicKeyB := privateKeyB.PublicKey(keys.PublicKeyWeightThreshold)

		accountAddressA := b.RootAccountAddress()
		accountAddressB, err := b.CreateAccount([]flow.AccountPublicKey{publicKeyB}, nil, getNonce())
		assert.Nil(t, err)

		multipleAccountScript := []byte(`
			pub fun main(accountA: Account, accountB: Account) {
				log(accountA.address)
				log(accountB.address)
			}
		`)

		tx := flow.Transaction{
			Script:             multipleAccountScript,
			ReferenceBlockHash: nil,
			Nonce:              getNonce(),
			ComputeLimit:       10,
			PayerAccount:       accountAddressA,
			ScriptAccounts:     []flow.Address{accountAddressA, accountAddressB},
		}

		sigA, err := keys.SignTransaction(tx, privateKeyA)
		assert.Nil(t, err)

		sigB, err := keys.SignTransaction(tx, privateKeyB)
		assert.Nil(t, err)

		tx.AddSignature(accountAddressA, sigA)
		tx.AddSignature(accountAddressB, sigB)

		err = b.SubmitTransaction(tx)
		assert.Nil(t, err)

		assert.Contains(t, loggedMessages, fmt.Sprintf(`"%x"`, accountAddressA.Bytes()))
		assert.Contains(t, loggedMessages, fmt.Sprintf(`"%x"`, accountAddressB.Bytes()))
	})
}

func TestGetTransaction(t *testing.T) {
	b := emulator.NewEmulatedBlockchain(emulator.DefaultOptions)

	myEventType := types.Event{
		FieldTypes: []types.EventField{
			{
				Identifier: "x",
				Type:       types.Int{},
			},
		},
	}

	eventsScript := `
		event MyEvent(x: Int)

		pub fun main(account: Account) {
			emit MyEvent(x: 1)	
		}
	`

	tx := flow.Transaction{
		Script:         []byte(eventsScript),
		Nonce:          getNonce(),
		ComputeLimit:   10,
		PayerAccount:   b.RootAccountAddress(),
		ScriptAccounts: []flow.Address{b.RootAccountAddress()},
	}

	sig, err := keys.SignTransaction(tx, b.RootKey())
	assert.Nil(t, err)

	tx.AddSignature(b.RootAccountAddress(), sig)

	err = b.SubmitTransaction(tx)
	assert.Nil(t, err)

	t.Run("InvalidHash", func(t *testing.T) {
		_, err := b.GetTransaction(crypto.Hash{})
		if assert.Error(t, err) {
			fmt.Println(err.Error())
			assert.IsType(t, nil, nil)
		}
	})

	t.Run("ValidHash", func(t *testing.T) {
		resTx, err := b.GetTransaction(tx.Hash())
		require.Nil(t, err)

		assert.Equal(t, resTx.Status, flow.TransactionFinalized)
		assert.Len(t, resTx.Events, 1)

		actualEvent := resTx.Events[0]

		eventValue, err := encoding.Decode(myEventType, actualEvent.Payload)
		require.Nil(t, err)

		decodedEvent := eventValue.(values.Event)

		eventType := fmt.Sprintf("tx.%s.MyEvent", tx.Hash().Hex())

		assert.Equal(t, tx.Hash(), actualEvent.TxHash)
		assert.Equal(t, eventType, actualEvent.Type)
		assert.Equal(t, uint(0), actualEvent.Index)
		assert.Equal(t, values.NewInt(1), decodedEvent.Fields[0])
	})
}

package virtualmachine_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	vmMock "github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine/mock"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hash"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestTransactionASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()
	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)
	bc := vm.NewBlockContext(&h, new(vmMock.Blocks))

	t.Run("transaction execution results in cached program", func(t *testing.T) {
		tx := &flow.TransactionBody{
			Authorizers: []flow.Address{unittest.AddressFixture()},
			Script: []byte(`
                transaction {
                  prepare(signer: AuthAccount) {}
                }
            `),
		}

		err := testutil.SignTransactionByRoot(tx, 0)
		require.NoError(t, err)

		ledger := testutil.RootBootstrappedLedger()

		result, err := bc.ExecuteTransaction(ledger, tx)
		require.NoError(t, err)

		require.True(t, result.Succeeded())
		require.Nil(t, result.Error)

		// Determine location of transaction
		txID := tx.ID()
		location := runtime.TransactionLocation(txID[:])

		// Get cached program
		program, err := vm.ASTCache().GetProgram(location)
		require.NotNil(t, program)
		require.NoError(t, err)
	})

}

func TestScriptASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()
	vm, err := virtualmachine.New(rt)

	require.NoError(t, err)
	bc := vm.NewBlockContext(&h, new(vmMock.Blocks))

	t.Run("script execution results in cached program", func(t *testing.T) {
		script := []byte(`
			pub fun main(): Int {
				return 42
			}
		`)

		ledger := testutil.RootBootstrappedLedger()

		result, err := bc.ExecuteScript(ledger, script)
		require.NoError(t, err)
		require.True(t, result.Succeeded())

		// Determine location
		scriptHash := hash.DefaultHasher.ComputeHash(script)
		location := runtime.ScriptLocation(scriptHash)

		// Get cached program
		program, err := vm.ASTCache().GetProgram(location)
		require.NotNil(t, program)
		require.NoError(t, err)

	})
}

func TestTransactionWithProgramASTCache(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)
	bc := vm.NewBlockContext(&h, new(vmMock.Blocks))

	// Create a number of account private keys.
	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(t, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger := testutil.RootBootstrappedLedger()
	accounts, err := testutil.CreateAccounts(vm, ledger, privateKeys)
	require.NoError(t, err)

	// Create deployment transaction that imports the FlowToken contract
	useImportTx := flow.NewTransactionBody().
		SetScript([]byte(fmt.Sprintf(`
				import FlowToken from 0x%s
				transaction {
					prepare(signer: AuthAccount) {}
					execute {
						let v <- FlowToken.createEmptyVault()
						destroy v
					}
				}
			`, virtualmachine.FlowTokenAddress())),
		).
		AddAuthorizer(accounts[0]).
		SetProposalKey(accounts[0], 0, 0).
		SetPayer(flow.ServiceAddress())

	err = testutil.SignPayload(useImportTx, accounts[0], privateKeys[0])
	require.NoError(t, err)

	err = testutil.SignEnvelope(useImportTx, flow.ServiceAddress(), unittest.ServiceAccountPrivateKey)
	require.NoError(t, err)

	// Run the Use import (FT Vault resource) transaction
	result, err := bc.ExecuteTransaction(ledger, useImportTx)
	require.NoError(t, err)

	if !assert.Nil(t, result.Error) {
		t.Fatal(result.Error.ErrorMessage())
	}

	// Determine location of transaction
	txID := useImportTx.ID()
	location := runtime.TransactionLocation(txID[:])

	// Get cached program
	program, err := vm.ASTCache().GetProgram(location)
	require.NotNil(t, program)
	require.NoError(t, err)
}

func BenchmarkTransactionWithProgramASTCache(b *testing.B) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	require.NoError(b, err)
	bc := vm.NewBlockContext(&h, new(vmMock.Blocks))

	// Create a number of account private keys.
	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(b, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger := testutil.RootBootstrappedLedger()
	accounts, err := testutil.CreateAccounts(vm, ledger, privateKeys)
	require.NoError(b, err)

	// Create many transactions that import the FlowToken contract.
	var txs []*flow.TransactionBody

	for i := 0; i < 1000; i++ {
		tx := flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(`
				import FlowToken from 0x%s
				transaction {
					prepare(signer: AuthAccount) {}
					execute {
						log("Transaction %d")
						let v <- FlowToken.createEmptyVault()
						destroy v
					}
				}
			`, virtualmachine.FlowTokenAddress(), i)),
			).
			AddAuthorizer(accounts[0]).
			SetProposalKey(accounts[0], 0, uint64(i)).
			SetPayer(flow.ServiceAddress())

		err = testutil.SignPayload(tx, accounts[0], privateKeys[0])
		require.NoError(b, err)

		err = testutil.SignEnvelope(tx, flow.ServiceAddress(), unittest.ServiceAccountPrivateKey)
		require.NoError(b, err)

		txs = append(txs, tx)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, tx := range txs {
			// Run the Use import (FT Vault resource) transaction.
			result, err := bc.ExecuteTransaction(ledger, tx)
			require.NoError(b, err)

			if !assert.Nil(b, result.Error) {
				b.Fatal(result.Error.ErrorMessage())
			}
		}
	}

}

type nonFunctioningCache struct{}

func (cache *nonFunctioningCache) GetProgram(location ast.Location) (*ast.Program, error) {
	return nil, nil
}

func (cache *nonFunctioningCache) SetProgram(location ast.Location, program *ast.Program) error {
	return nil
}

func BenchmarkTransactionWithoutProgramASTCache(b *testing.B) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.NewWithCache(rt, &nonFunctioningCache{})
	require.NoError(b, err)
	bc := vm.NewBlockContext(&h, new(vmMock.Blocks))

	// Create a number of account private keys.
	privateKeys, err := testutil.GenerateAccountPrivateKeys(1)
	require.NoError(b, err)

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger := testutil.RootBootstrappedLedger()
	accounts, err := testutil.CreateAccounts(vm, ledger, privateKeys)
	require.NoError(b, err)

	// Create many transactions that import the FlowToken contract.
	var txs []*flow.TransactionBody

	for i := 0; i < 1000; i++ {
		tx := flow.NewTransactionBody().
			SetScript([]byte(fmt.Sprintf(`
				import FlowToken from 0x%s
				transaction {
					prepare(signer: AuthAccount) {}
					execute {
						log("Transaction %d")
						let v <- FlowToken.createEmptyVault()
						destroy v
					}
				}
			`, virtualmachine.FlowTokenAddress(), i)),
			).
			AddAuthorizer(accounts[0]).
			SetPayer(accounts[0]).
			SetProposalKey(accounts[0], 0, uint64(i))

		err = testutil.SignEnvelope(tx, accounts[0], privateKeys[0])
		require.NoError(b, err)

		txs = append(txs, tx)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		for _, tx := range txs {
			// Run the Use import (FT Vault resource) transaction.
			result, err := bc.ExecuteTransaction(ledger, tx)
			require.True(b, result.Succeeded())
			require.NoError(b, err)
		}
	}
}

func TestProgramASTCacheAvoidRaceCondition(t *testing.T) {
	rt := runtime.NewInterpreterRuntime()
	h := unittest.BlockHeaderFixture()

	vm, err := virtualmachine.New(rt)
	require.NoError(t, err)
	bc := vm.NewBlockContext(&h, new(vmMock.Blocks))

	// Bootstrap a ledger, creating accounts with the provided private keys and the root account.
	ledger := testutil.RootBootstrappedLedger()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int, wg *sync.WaitGroup) {
			defer wg.Done()
			view := delta.NewView(ledger.Get)
			result, err := bc.ExecuteScript(view, []byte(fmt.Sprintf(`
				import FlowToken from 0x%s
				pub fun main() {
					log("Script %d")
					let v <- FlowToken.createEmptyVault()
					destroy v
				}
			`, virtualmachine.FlowTokenAddress(), id)))
			if !assert.True(t, result.Succeeded()) {
				t.Log(result.Error.ErrorMessage())
			}
			require.NoError(t, err)
			require.True(t, result.Succeeded())
		}(i, &wg)
	}
	wg.Wait()

	location := runtime.AddressLocation(virtualmachine.FlowTokenAddress().Bytes())

	// Get cached program
	program, err := vm.ASTCache().GetProgram(location)
	require.NotNil(t, program)
	require.NoError(t, err)
}

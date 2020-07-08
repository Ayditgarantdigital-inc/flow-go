package common

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/dapperlabs/flow-go/engine/execution/testutil"
	"github.com/dapperlabs/flow-go/engine/ghost/client"
	"github.com/dapperlabs/flow-go/integration/convert"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/dsl"
)

var (
	// CounterContract is a simple counter contract in Cadence
	CounterContract = dsl.Contract{
		Name: "Testing",
		Members: []dsl.CadenceCode{
			dsl.Resource{
				Name: "Counter",
				Code: `
				pub var count: Int

				init() {
					self.count = 0
				}
				pub fun add(_ count: Int) {
					self.count = self.count + count
				}`,
			},
			dsl.Code(`
				pub fun createCounter(): @Counter {
					return <-create Counter()
				}`,
			),
		},
	}
)

// CreateCounterTx is a transaction script for creating an instance of the counter in the account storage of the
// authorizing account NOTE: the counter contract must be deployed first
func CreateCounterTx(counterAddress flow.Address) dsl.Transaction {
	return dsl.Transaction{
		Import: dsl.Import{Address: counterAddress},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				var maybeCounter <- signer.load<@Testing.Counter>(from: /storage/counter)

				if maybeCounter == nil {
					maybeCounter <-! Testing.createCounter()
				}

				maybeCounter?.add(2)
				signer.save(<-maybeCounter!, to: /storage/counter)

				signer.link<&Testing.Counter>(/public/counter, target: /storage/counter)
				`),
		},
	}
}

// ReadCounterScript is a read-only script for reading the current value of the counter contract
func ReadCounterScript(contractAddress flow.Address, accountAddress flow.Address) dsl.Main {
	return dsl.Main{
		Import: dsl.Import{
			Names:   []string{"Testing"},
			Address: contractAddress,
		},
		ReturnType: "Int",
		Code: fmt.Sprintf(`
			let account = getAccount(0x%s)
			if let cap = account.getCapability(/public/counter) {
				return cap.borrow<&Testing.Counter>()?.count ?? -3
			}
			return -3`, accountAddress.Hex()),
	}
}

// CreateCounterPanicTx is a transaction script that creates a counter instance in the root account, but panics after
// manipulating state. It can be used to test whether execution state stays untouched/will revert. NOTE: the counter
// contract must be deployed first
func CreateCounterPanicTx(chain flow.Chain) dsl.Transaction {
	return dsl.Transaction{
		Import: dsl.Import{Address: chain.ServiceAddress()},
		Content: dsl.Prepare{
			Content: dsl.Code(`
				var maybeCounter <- signer.load<@Testing.Counter>(from: /storage/counter)

				if maybeCounter == nil {
					maybeCounter <-! Testing.createCounter()
				}

				maybeCounter?.add(2)
				signer.save(<-maybeCounter!, to: /storage/counter)

				signer.link<&Testing.Counter>(/public/counter, target: /storage/counter)

				panic("fail for testing purposes")
				`),
		},
	}
}

func createAccount(ctx context.Context, client *testnet.Client, root *flow.Block, code []byte, key flow.AccountPublicKey) error {

	var createAccountScript = []byte(`
	transaction(code: [Int], key: [Int]) {
		prepare(signer: AuthAccount) {
			let acct = AuthAccount(payer: signer)

			acct.setCode(code)
			acct.addPublicKey(key)
		}
	}
	`)

	encAccountKey, err := flow.EncodeRuntimeAccountPublicKey(key)
	if err != nil {
		return err
	}
	cadAccountKey := testutil.BytesToCadenceArray(encAccountKey)
	encCadAccountKey, err := jsoncdc.Encode(cadAccountKey)
	if err != nil {
		return err
	}

	cadCode := testutil.BytesToCadenceArray(code)
	encCadCode, err := jsoncdc.Encode(cadCode)
	if err != nil {
		return err
	}

	tx := flow.NewTransactionBody().
		SetScript([]byte(createAccountScript)).
		SetReferenceBlockID(root.ID()).
		SetProposalKey(client.Chain.ServiceAddress(), 0, client.GetSeqNumber()).
		SetPayer(client.Chain.ServiceAddress()).
		AddAuthorizer(client.Chain.ServiceAddress()).
		AddArgument(encCadCode).
		AddArgument(encCadAccountKey)

	err = client.SignAndSendTransaction(ctx, *tx)
	if err != nil {
		return err
	}

	return nil
}

// readCounter executes a script to read the value of a counter. The counter
// must have been deployed and created.
func readCounter(ctx context.Context, client *testnet.Client, address flow.Address) (int, error) {

	res, err := client.ExecuteScript(ctx, ReadCounterScript(address, address))
	if err != nil {
		return 0, err
	}

	v, err := jsoncdc.Decode(res)
	if err != nil {
		return 0, err
	}

	return v.(cadence.Int).Int(), nil
}

func GetGhostClient(ghostContainer *testnet.Container) (*client.GhostClient, error) {

	if !ghostContainer.Config.Ghost {
		return nil, fmt.Errorf("container is a not a ghost node container")
	}

	ghostPort, ok := ghostContainer.Ports[testnet.GhostNodeAPIPort]
	if !ok {
		return nil, fmt.Errorf("ghost node API port not found")
	}

	addr := fmt.Sprintf(":%s", ghostPort)

	return client.NewGhostClient(addr)
}

// GetAccount returns a new account address, key, and signer.
func GetAccount(chain flow.Chain) (sdk.Address, *sdk.AccountKey, sdkcrypto.Signer) {

	addr := convert.ToSDKAddress(chain.ServiceAddress())

	key := RandomPrivateKey()
	signer := sdkcrypto.NewInMemorySigner(key, sdkcrypto.SHA3_256)

	acct := sdk.NewAccountKey().
		FromPrivateKey(key).
		SetHashAlgo(sdkcrypto.SHA3_256).
		SetWeight(sdk.AccountKeyWeightThreshold)

	return addr, acct, signer
}

// RandomPrivateKey returns a randomly generated ECDSA P-256 private key.
func RandomPrivateKey() sdkcrypto.PrivateKey {
	seed := make([]byte, sdkcrypto.MinSeedLength)

	_, err := rand.Read(seed)
	if err != nil {
		panic(err)
	}

	privateKey, err := sdkcrypto.GeneratePrivateKey(sdkcrypto.ECDSA_P256, seed)
	if err != nil {
		panic(err)
	}

	return privateKey
}

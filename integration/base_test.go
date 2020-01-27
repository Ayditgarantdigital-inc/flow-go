package integration_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"

	sdk "github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/client"
	"github.com/dapperlabs/flow-go-sdk/keys"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
)

func TestContainer_Start(t *testing.T) {

	nonce := 2137

	tx := sdk.Transaction{
		Script:             []byte("fun main() {}"),
		ReferenceBlockHash: []byte{1, 2, 3, 4},
		Nonce:              uint64(nonce),
		ComputeLimit:       10,
		PayerAccount:       sdk.RootAddress,
	}

	b := tx.Encode()

	fmt.Printf("bytes: %X\n", b)
	fmt.Printf("bytes: %s\n", b)

	hasher, err := crypto.NewHasher(crypto.SHA2_256)
	require.NoError(t, err)

	hash := hasher.ComputeHash(b)

	fmt.Printf("hash: %X\n", hash)
	fmt.Printf("hash: %s\n", hash)

	//net := []*network.FlowNode{
	//	{
	//		Role:  flow.RoleCollection,
	//		Stake: 1000,
	//	},
	//	{
	//		Role:  flow.RoleConsensus,
	//		Stake: 1000,
	//	},
	//	{
	//		Role:  flow.RoleExecution,
	//		Stake: 1234,
	//	},
	//	{
	//		Role:  flow.RoleVerification,
	//		Stake: 4582,
	//	},
	//}
	//
	//// Enable verbose logging
	//testingdock.Verbose = true
	//
	//ctx := context.Background()
	//
	//flowNetwork, err := network.PrepareFlowNetwork(ctx, t, "mvp", net)
	//require.NoError(t, err)
	//
	//flowNetwork.Suite.Start(ctx)
	//defer flowNetwork.Suite.Close()
	//
	//var collectionNodeApiPort = ""
	//for _, container := range flowNetwork.Containers {
	//	if container.Identity.Role == flow.RoleCollection {
	//		collectionNodeApiPort = container.Ports["api"]
	//	}
	//}
	//require.NotEqual(t, collectionNodeApiPort, "")
	//
	//sendTransaction(t, collectionNodeApiPort)
	//
	//// TODO Once we have observation API in place, query this API as the actual method of test assertion
	//time.Sleep(15 * time.Second)
}

func sendTransaction(t *testing.T, apiPort string) {
	fmt.Printf("Sending tx to %s\n", apiPort)
	c, err := client.New("localhost:" + apiPort)
	require.NoError(t, err)

	// Generate key
	seed := make([]byte, 40)
	_, _ = rand.Read(seed)
	key, err := keys.GeneratePrivateKey(keys.ECDSA_P256_SHA2_256, seed)
	require.NoError(t, err)

	nonce := 2137

	tx := sdk.Transaction{
		Script:             []byte("fun main() {}"),
		ReferenceBlockHash: []byte{1, 2, 3, 4},
		Nonce:              uint64(nonce),
		ComputeLimit:       10,
		PayerAccount:       sdk.RootAddress,
	}

	sig, err := keys.SignTransaction(tx, key)
	require.NoError(t, err)

	tx.AddSignature(sdk.RootAddress, sig)

	err = c.SendTransaction(context.Background(), tx)
	require.NoError(t, err)
}

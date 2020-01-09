package tests

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/stub"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func prepareNodesAndCollections(N, M int) (
	[]*mockPropagationNode, []*flow.CollectionGuarantee, error) {

	rand.Seed(time.Now().UnixNano())

	// prepare N connected nodes
	entries := make([]string, N)
	for e := 0; e < N; e++ {
		nodeID := unittest.IdentifierFixture()
		entries[e] = fmt.Sprintf("consensus-%s@address%d=1000", nodeID, e+1)
	}
	_, nodes, err := createConnectedNodes(entries...)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not create connected nodes")
	}

	// prepare M distinct collection hashes
	gcs := make([]*flow.CollectionGuarantee, M)
	for m := 0; m < M; m++ {
		gcs[m] = randCollectionGuarantee()
	}
	return nodes, gcs, nil
}

// given a list of node entries, return a list of mock nodes and connect them all to a hub
func createConnectedNodes(nodeEntries ...string) (*stub.Hub, []*mockPropagationNode, error) {
	if len(nodeEntries) == 0 {
		return nil, nil, errors.New("NodeEntries must not be empty")
	}

	identities := make(flow.IdentityList, 0, len(nodeEntries))
	for _, entry := range nodeEntries {
		id, err := flow.ParseIdentity(entry)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not parse identity (%s)", entry)
		}
		identities = append(identities, id)
	}

	header := flow.Header{
		Number:    0,
		Timestamp: time.Now().UTC(),
		Parent:    crypto.ZeroHash,
	}
	genesis := flow.Block{
		Header:        header,
		NewIdentities: identities,
	}
	genesis.Header.Payload = genesis.Payload()

	hub := stub.NewNetworkHub()

	nodes := make([]*mockPropagationNode, 0)
	for i := range nodeEntries {
		node, err := newMockPropagationNode(hub, &genesis, i)
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not create node (%d)", i)
		}
		nodes = append(nodes, node)
	}

	return hub, nodes, nil
}

// a utility func to return a random collection hash
func randHash() []byte {
	hash := make([]byte, 32)
	_, err := rand.Read(hash)
	if err != nil {
		panic(err.Error()) // should never error
	}
	return hash
}

// a utility func to generate a CollectionGuarantee with random hash
func randCollectionGuarantee() *flow.CollectionGuarantee {
	hash := randHash()
	return &flow.CollectionGuarantee{
		Hash: hash,
	}
}

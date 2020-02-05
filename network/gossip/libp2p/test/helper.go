package test

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/network/codec/json"
	"github.com/dapperlabs/flow-go/network/gossip/libp2p"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
)

// helper offers a set of functions that are shared among different tests
// CreateIDs creates and initializes count-many flow identifiers instances
func CreateIDs(count int) []*flow.Identity {
	identities := make([]*flow.Identity, 0)
	for i := 0; i < count; i++ {
		// defining id of node
		var nodeID [32]byte
		nodeID[0] = byte(i + 1)
		identity := &flow.Identity{
			NodeID: nodeID,
		}
		identities = append(identities, identity)
	}
	return identities
}

// createNetworks receives a slice of middlewares their associated flow identifiers,
// and for each middleware creates a network instance on top
// it returns the slice of created middlewares
// csize is the receive cache size of the nodes
func CreateNetworks(mws []*libp2p.Middleware, ids flow.IdentityList, csize int, dryrun bool) ([]*libp2p.Network, error) {
	count := len(mws)
	nets := make([]*libp2p.Network, 0)

	// creates and mocks the state
	var snapshot *SnapshotMock
	if dryrun {
		snapshot = &SnapshotMock{ids: ids}
	} else {
		snapshot = &SnapshotMock{ids: flow.IdentityList{}}
	}
	state := &protocol.State{}
	state.On("Final").Return(snapshot)

	for i := 0; i < count; i++ {
		// creates and mocks me
		me := &mock.Local{}
		me.On("NodeID").Return(ids[i].NodeID)
		net, err := libp2p.NewNetwork(zerolog.Logger{}, json.NewCodec(), state, me, mws[i], csize)
		if err != nil {
			return nil, fmt.Errorf("could not create error %w", err)
		}

		nets = append(nets, net)
	}

	// if dryrun then don't actually start the network just return the network objects
	if dryrun {
		return nets, nil
	}

	for _, net := range nets {
		<-net.Ready()
	}

	for i, m := range mws {
		// retrieves IP and port of the middleware
		ip, port := m.GetIPPort()

		// mocks an identity for the middleware
		id := &flow.Identity{
			NodeID:  ids[i].NodeID,
			Address: fmt.Sprintf("%s:%s", ip, port),
			Role:    flow.RoleCollection,
			Stake:   0,
		}
		snapshot.ids = append(snapshot.ids, id)
	}

	return nets, nil
}

// CreateMiddleware receives an ids slice and creates and initializes a middleware instances for each id
func CreateMiddleware(identities []*flow.Identity) ([]*libp2p.Middleware, error) {
	count := len(identities)
	mws := make([]*libp2p.Middleware, 0)
	for i := 0; i < count; i++ {
		// creating middleware of nodes
		mw, err := libp2p.NewMiddleware(zerolog.Logger{}, json.NewCodec(), "0.0.0.0:0", identities[i].NodeID)
		if err != nil {
			return nil, err
		}

		mws = append(mws, mw)
	}
	return mws, nil
}

type SnapshotMock struct {
	ids flow.IdentityList
}

func (s *SnapshotMock) Identities(filters ...flow.IdentityFilter) (flow.IdentityList, error) {
	return s.ids, nil
}

func (s *SnapshotMock) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	return nil, fmt.Errorf(" not implemented")
}

func (s *SnapshotMock) Commit() (flow.StateCommitment, error) {
	return nil, fmt.Errorf(" not implemented")
}

func (s *SnapshotMock) Clusters() (*flow.ClusterList, error) {
	return nil, fmt.Errorf(" not implemented")
}

func (s *SnapshotMock) Head() (*flow.Header, error) {
	return nil, fmt.Errorf(" not implemented")
}

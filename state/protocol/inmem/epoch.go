package inmem

import (
	"fmt"

	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/flow/order"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/invalid"
	"github.com/onflow/flow-go/state/protocol/seed"
)

// Epoch is a memory-backed implementation of protocol.Epoch.
type Epoch struct {
	enc EncodableEpoch
}

func (e Epoch) Counter() (uint64, error)   { return e.enc.Counter, nil }
func (e Epoch) FirstView() (uint64, error) { return e.enc.FirstView, nil }
func (e Epoch) FinalView() (uint64, error) { return e.enc.FinalView, nil }
func (e Epoch) InitialIdentities() (flow.IdentityList, error) {
	return e.enc.InitialIdentities, nil
}
func (e Epoch) RandomSource() ([]byte, error) { return e.enc.RandomSource, nil }

func (e Epoch) Seed(indices ...uint32) ([]byte, error) {
	return seed.FromRandomSource(indices, e.enc.RandomSource)
}

func (e Epoch) Clustering() (flow.ClusterList, error) {
	return e.enc.Clustering, nil
}

func (e Epoch) DKG() (protocol.DKG, error) {
	if e.enc.DKG != nil {
		return DKG{*e.enc.DKG}, nil
	}
	return nil, protocol.ErrEpochNotCommitted
}

func (e Epoch) Cluster(i uint) (protocol.Cluster, error) {
	if e.enc.Clusters != nil {
		if i >= uint(len(e.enc.Clusters)) {
			return nil, fmt.Errorf("no cluster with index %d", i)
		}
		return Cluster{e.enc.Clusters[i]}, nil
	}
	return nil, protocol.ErrEpochNotCommitted
}

type Epochs struct {
	enc EncodableEpochs
}

func (eq Epochs) Previous() protocol.Epoch {
	if eq.enc.Previous != nil {
		return Epoch{*eq.enc.Previous}
	}
	return invalid.NewEpoch(protocol.ErrNoPreviousEpoch)
}
func (eq Epochs) Current() protocol.Epoch {
	return Epoch{eq.enc.Current}
}
func (eq Epochs) Next() protocol.Epoch {
	if eq.enc.Next != nil {
		return Epoch{*eq.enc.Next}
	}
	return invalid.NewEpoch(protocol.ErrNextEpochNotSetup)
}

// setupEpoch is an implementation of protocol.Epoch backed by an EpochSetup
// service event. This is used for converting service events to inmem.Epoch.
type setupEpoch struct {
	// EpochSetup service event
	setupEvent *flow.EpochSetup
}

func (es *setupEpoch) Counter() (uint64, error) {
	return es.setupEvent.Counter, nil
}

func (es *setupEpoch) FirstView() (uint64, error) {
	return es.setupEvent.FirstView, nil
}

func (es *setupEpoch) FinalView() (uint64, error) {
	return es.setupEvent.FinalView, nil
}

func (es *setupEpoch) InitialIdentities() (flow.IdentityList, error) {
	identities := es.setupEvent.Participants.Filter(filter.Any)
	// apply a deterministic sort to the participants
	identities = identities.Order(order.ByNodeIDAsc)

	return identities, nil
}

func (es *setupEpoch) Clustering() (flow.ClusterList, error) {
	collectorFilter := filter.HasRole(flow.RoleCollection)
	clustering, err := flow.NewClusterList(es.setupEvent.Assignments, es.setupEvent.Participants.Filter(collectorFilter))
	if err != nil {
		return nil, fmt.Errorf("failed to generate ClusterList from collector identities: %w", err)
	}
	return clustering, nil
}

func (es *setupEpoch) Cluster(_ uint) (protocol.Cluster, error) {
	return nil, protocol.ErrEpochNotCommitted
}

func (es *setupEpoch) DKG() (protocol.DKG, error) {
	return nil, protocol.ErrEpochNotCommitted
}

func (es *setupEpoch) RandomSource() ([]byte, error) {
	return es.setupEvent.RandomSource, nil
}

func (es *setupEpoch) Seed(indices ...uint32) ([]byte, error) {
	return seed.FromRandomSource(indices, es.setupEvent.RandomSource)
}

// committedEpoch is an implementation of protocol.Epoch backed by an EpochSetup
// and EpochCommit service event. This is used for converting service events to
// inmem.Epoch.
type committedEpoch struct {
	setupEpoch
	commitEvent *flow.EpochCommit
}

func (es *committedEpoch) Cluster(index uint) (protocol.Cluster, error) {
	qcs := es.commitEvent.ClusterQCs
	if uint(len(qcs)) <= index {
		return nil, fmt.Errorf("no cluster with index %d", index)
	}
	rootQC := qcs[index]

	clustering, err := es.Clustering()
	if err != nil {
		return nil, fmt.Errorf("failed to generate clustering: %w", err)
	}

	members, ok := clustering.ByIndex(index)
	if !ok {
		return nil, fmt.Errorf("failed to get members of cluster %d: %w", index, err)
	}
	epochCounter := es.setupEvent.Counter

	cluster, err := ClusterFromEncodable(EncodableCluster{
		Index:     index,
		Counter:   epochCounter,
		Members:   members,
		RootBlock: cluster.CanonicalRootBlock(epochCounter, members),
		RootQC:    rootQC,
	})
	return cluster, err
}

func (es *committedEpoch) DKG() (protocol.DKG, error) {
	dkg, err := DKGFromEncodable(EncodableDKG{
		GroupKey: encodable.RandomBeaconPubKey{
			PublicKey: es.commitEvent.DKGGroupKey,
		},
		Participants: es.commitEvent.DKGParticipants,
	})
	return dkg, err
}

// NewSetupEpoch returns an memory-backed epoch implementation based on an
// EpochSetup event. Epoch information available after the setup phase will
// not be accessible in the resulting epoch instance.
func NewSetupEpoch(setupEvent *flow.EpochSetup) (*Epoch, error) {
	convertible := &setupEpoch{
		setupEvent: setupEvent,
	}
	return FromEpoch(convertible)
}

// NewSetupEpoch returns an memory-backed epoch implementation based on an
// EpochSetup and EpochCommit event.
func NewCommittedEpoch(setupEvent *flow.EpochSetup, commitEvent *flow.EpochCommit) (*Epoch, error) {
	convertible := &committedEpoch{
		setupEpoch: setupEpoch{
			setupEvent: setupEvent,
		},
		commitEvent: commitEvent,
	}
	return FromEpoch(convertible)
}

// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package round

import (
	"encoding/binary"
	"math/rand"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/protocol"
)

// Round keeps track of the current consensus state.
type Round struct {
	parent       *flow.Header
	leader       *flow.Identity
	quorum       uint64
	participants flow.IdentityList
	candidate    *flow.Block
	votes        map[flow.Identifier]uint64
}

// New creates a new consensus cache.
func New(state protocol.State, me module.Local) (*Round, error) {

	// retrieve what is currently the last finalized block
	head, err := state.Final().Head()
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve head")
	}

	// get participants to the consensus algorithm
	ids, err := state.Final().Identities(
		identity.HasRole(flow.RoleConsensus),
	)
	if err != nil {
		return nil, errors.Wrap(err, "could not retrieve identities")
	}

	// take first 8 bytes of previous block hash as a seed to shuffle identities
	headID := head.ID()
	seed := binary.LittleEndian.Uint64(headID[:])
	r := rand.New(rand.NewSource(int64(seed)))
	r.Shuffle(len(ids), func(i int, j int) {
		ids[i], ids[j] = ids[j], ids[i]
	})

	// remove ourselves from the participants
	leader := ids[0]
	quorum := ids.TotalStake()
	participants := ids.Filter(identity.Not(identity.HasNodeID(me.NodeID())))

	s := &Round{
		parent:       head,
		leader:       leader,
		quorum:       quorum,
		participants: participants,
		votes:        make(map[flow.Identifier]uint64),
	}
	return s, nil
}

// Parent returns the parent of the to-be-formed block.
func (r *Round) Parent() *flow.Header {
	return r.parent
}

// Participants will retrieve cached identities for this round.
func (r *Round) Participants() flow.IdentityList {
	return r.participants
}

// Quorum returns the quorum for a qualified majority in this round.
func (r *Round) Quorum() uint64 {
	return r.quorum
}

// Leader returns the the leader of the current round.
func (r *Round) Leader() *flow.Identity {
	return r.leader
}

// Propose sets the current candidate header.
func (r *Round) Propose(candidate *flow.Block) {
	r.candidate = candidate
}

// Candidate returns the current candidate header.
func (r *Round) Candidate() *flow.Block {
	return r.candidate
}

// Voted checks if the given node has already voted.
func (r *Round) Voted(nodeID flow.Identifier) bool {
	_, ok := r.votes[nodeID]
	return ok
}

// Tally will add the given vote to the node tally.
func (r *Round) Tally(voterID flow.Identifier, stake uint64) {
	r.votes[voterID] = stake
}

// Votes will count the total votes.
func (r *Round) Votes() uint64 {
	var votes uint64
	for _, stake := range r.votes {
		votes += stake
	}
	return votes
}

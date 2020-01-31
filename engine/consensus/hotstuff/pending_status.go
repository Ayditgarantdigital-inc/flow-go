package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// PendingStatus keeps track of pending votes
type PendingStatus struct {
	// When receiving missing block, first received votes will be accumulated
	orderedVotes []*types.Vote
	// For avoiding duplicate votes
	voteMap map[string]*types.Vote
}

func (ps *PendingStatus) AddVote(vote *types.Vote) {
	ps.voteMap[string(vote.ID())] = vote
	ps.orderedVotes = append(ps.orderedVotes, vote)
}

func NewPendingStatus() *PendingStatus {
	return &PendingStatus{
		voteMap: map[string]*types.Vote{},
	}
}

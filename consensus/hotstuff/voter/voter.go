package voter

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// Voter produces votes for the given block
type Voter struct {
	signer        hotstuff.SignerVerifier
	forks         hotstuff.ForksReader
	persist       hotstuff.Persister
	committee     hotstuff.Committee // only produce votes when we are valid committee members
	lastVotedView uint64             // need to keep track of the last view we voted for so we don't double vote accidentally
}

// New creates a new Voter instance
func New(
	signer hotstuff.SignerVerifier,
	forks hotstuff.ForksReader,
	persist hotstuff.Persister,
	committee hotstuff.Committee,
	lastVotedView uint64,
) *Voter {

	return &Voter{
		signer:        signer,
		forks:         forks,
		persist:       persist,
		committee:     committee,
		lastVotedView: lastVotedView,
	}
}

// ProduceVoteIfVotable will make a decision on whether it will vote for the given proposal, the returned
// error indicates whether to vote or not.
// In order to ensure that only a safe node will be voted, Voter will ask Forks whether a vote is a safe node or not.
// The curView is taken as input to ensure Voter will only vote for proposals at current view and prevent double voting.
// This method will only ever _once_ return a `non-nil vote, nil` vote: the very first time it encounters a safe block of the
//  current view to vote for. Subsequently, voter does _not_ vote for any other block with the same (or lower) view.
// (including repeated calls with the initial block we voted for also return `nil, error`).
func (v *Voter) ProduceVoteIfVotable(block *model.Block, curView uint64) (*model.Vote, error) {
	if !v.forks.IsSafeBlock(block) {
		return nil, model.NoVoteError{Msg: "not safe block"}
	}

	if curView != block.View {
		return nil, model.NoVoteError{Msg: "not for current view"}
	}

	if curView <= v.lastVotedView {
		return nil, model.NoVoteError{Msg: "not above the last voted view"}
	}

	// Do not produce a vote for blocks where we are not a valid committee
	// member. HotStuff will ask for a vote for the first block of the next epoch, even if we are unstaked in 
	// the next epoch.
	// These votes can't be used to produce valid QCs.
	_, err := v.committee.Identity(block.BlockID, v.committee.Self())
	if errors.Is(model.ErrInvalidSigner, err) {
		return nil, model.NoVoteError{Msg: "not voting committee member for block"}
	}
	if err != nil {
		return nil, fmt.Errorf("could not get self identity: %w", err)
	}

	vote, err := v.signer.CreateVote(block)
	if err != nil {
		return nil, fmt.Errorf("could not vote for block: %w", err)
	}

	// vote for the current view has been produced, update lastVotedView
	// to prevent from voting for the same view again
	v.lastVotedView = curView
	err = v.persist.PutVoted(curView)
	if err != nil {
		return nil, fmt.Errorf("could not persist last voted: %w", err)
	}

	return vote, nil
}

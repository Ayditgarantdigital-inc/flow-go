package hotstuff

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

type VoteAggregator struct {
	logger                zerolog.Logger
	notifier              notifications.Consumer
	viewState             *ViewState
	voteValidator         *Validator
	sigAggregator         SigAggregator
	highestPrunedView     uint64
	pendingVotes          *PendingVotes                                   // keeps track of votes whose blocks can not be found
	viewToBlockIDSet      map[uint64]map[flow.Identifier]struct{}         // for pruning
	viewToVoteID          map[uint64]map[flow.Identifier]*hotstuff.Vote   // for detecting double voting
	createdQC             map[flow.Identifier]*hotstuff.QuorumCertificate // keeps track of QCs that have been made for blocks
	blockIDToVotingStatus map[flow.Identifier]*VotingStatus               // keeps track of accumulated votes and stakes for blocks
	proposerVotes         map[flow.Identifier]*hotstuff.Vote              // holds the votes of block proposers, so we can avoid passing around proposals everywhere
}

// NewVoteAggregator creates an instance of vote aggregator
func NewVoteAggregator(notifier notifications.Consumer, highestPrunedView uint64, viewState *ViewState, voteValidator *Validator, sigAggregator SigAggregator) *VoteAggregator {
	return &VoteAggregator{
		// TODO: implement logger in the future
		logger:                zerolog.Logger{},
		notifier:              notifier,
		highestPrunedView:     highestPrunedView,
		viewState:             viewState,
		voteValidator:         voteValidator,
		sigAggregator:         sigAggregator,
		pendingVotes:          NewPendingVotes(),
		viewToBlockIDSet:      make(map[uint64]map[flow.Identifier]struct{}),
		viewToVoteID:          make(map[uint64]map[flow.Identifier]*hotstuff.Vote),
		createdQC:             make(map[flow.Identifier]*hotstuff.QuorumCertificate),
		blockIDToVotingStatus: make(map[flow.Identifier]*VotingStatus),
		proposerVotes:         make(map[flow.Identifier]*hotstuff.Vote),
	}
}

// StorePendingVote stores the vote as a pending vote assuming the caller has checked that the voting
// block is currently missing.
// Note: Validations on these pending votes will be postponed until the block has been received.
func (va *VoteAggregator) StorePendingVote(vote *hotstuff.Vote) bool {
	// check if the vote is for a view that has already been pruned (and is thus stale)
	// cannot store vote for already pruned view
	if va.isStale(vote) {
		return false
	}
	// add vote, return false if the vote exists
	exists := va.pendingVotes.AddVote(vote)
	if exists {
		return false
	}
	va.updateState(vote)
	return true
}

// StoreVoteAndBuildQC stores the vote assuming the caller has checked that the voting block is incorporated,
// and returns a QC if there are votes with enough stakes.
// The VoteAggregator builds a QC as soon as the number of votes allow this.
// While subsequent votes (past the required threshold) are not included in the QC anymore,
// VoteAggregator ALWAYS returns the same QC as the one returned before.
func (va *VoteAggregator) StoreVoteAndBuildQC(vote *hotstuff.Vote, block *hotstuff.Block) (*hotstuff.QuorumCertificate, bool, error) {
	// if the QC for the block has been created before, return the QC
	oldQC, built := va.createdQC[block.BlockID]
	if built {
		return oldQC, true, nil
	}
	// ignore stale votes
	if va.isStale(vote) {
		return nil, false, hotstuff.StaleVoteError{Vote: vote, HighestPrunedView: va.highestPrunedView}
	}
	// validate the vote and adding it to the accumulated voting status
	valid, err := va.validateAndStoreIncorporatedVote(vote, block)
	if err != nil {
		return nil, false, fmt.Errorf("could not store incorporated vote: %w", err)
	}
	// cannot build qc if vote is invalid
	if !valid {
		return nil, false, nil
	}
	// try to build the QC with existing votes
	newQC, built, err := va.tryBuildQC(block.BlockID)
	if err != nil {
		return nil, false, fmt.Errorf("could not build QC: %w", err)
	}

	return newQC, built, nil
}

// StoreProposerVote stores the vote for a block that was proposed.
func (va *VoteAggregator) StoreProposerVote(vote *hotstuff.Vote) bool {
	// check if the proposer vote is for a view that has already been pruned (and is thus stale)
	if va.isStale(vote) { // cannot store vote for already pruned view
		return false
	}
	// add proposer vote, return false if it exists
	_, exists := va.proposerVotes[vote.BlockID]
	if exists {
		return false
	}
	va.proposerVotes[vote.BlockID] = vote
	return true
}

// BuildQCOnReceivedBlock will attempt to build a QC for the given block when there are votes
// with enough stakes.
// It assumes that the proposer's vote has been stored by calling StoreProposerVote
func (va *VoteAggregator) BuildQCOnReceivedBlock(block *hotstuff.Block) (*hotstuff.QuorumCertificate, bool, error) {
	// return the QC that was built before if exists
	oldQC, exists := va.createdQC[block.BlockID]
	if exists {
		return oldQC, true, nil
	}
	// proposer vote is the first to be accumulated
	proposerVote, exists := va.proposerVotes[block.BlockID]
	if !exists { // cannot build qc if proposer vote does not exist
		return nil, false, fmt.Errorf("could not get proposer vote for block: %x", block.BlockID)
	}
	if va.isStale(proposerVote) {
		return nil, false, nil
	}

	// accumulate leader vote first to ensure leader's vote is always included in the QC
	valid, err := va.validateAndStoreIncorporatedVote(proposerVote, block)
	if err != nil {
		return nil, false, fmt.Errorf("could not validate proposer vote: %w", err)
	}
	if !valid {
		// proposer vote should not be invalid unless validating block has failed
		return nil, false, fmt.Errorf("validating block failed: %x", block.BlockID)
	}
	// accumulate pending votes by order
	pendingStatus, exists := va.pendingVotes.votes[block.BlockID]
	if exists {
		err = va.convertPendingVotes(pendingStatus.orderedVotes, block)
		if err != nil {
			return nil, false, fmt.Errorf("could not build QC on receiving block: %w", err)
		}
	}
	// try building QC with existing valid votes
	qc, built, err := va.tryBuildQC(block.BlockID)
	if err != nil {
		return nil, false, fmt.Errorf("could not build QC on receiving block: %w", err)
	}
	delete(va.proposerVotes, block.BlockID)
	return qc, built, nil
}

func (va *VoteAggregator) convertPendingVotes(pendingVotes []*hotstuff.Vote, block *hotstuff.Block) error {
	for _, vote := range pendingVotes {
		valid, err := va.validateAndStoreIncorporatedVote(vote, block)
		if err != nil {
			return fmt.Errorf("processing pending votes failed: %w", err)
		}
		if !valid {
			continue
		}
		// if threshold is reached, the rest of the votes can be ignored
		if va.canBuildQC(block.BlockID) {
			break
		}
	}
	delete(va.pendingVotes.votes, block.BlockID)

	return nil
}

// PruneByView will delete all votes equal or below to the given view, as well as related indexes.
func (va *VoteAggregator) PruneByView(view uint64) {
	if view <= va.highestPrunedView {
		return
	}
	for i := va.highestPrunedView + 1; i <= view; i++ {
		blockIDStrSet := va.viewToBlockIDSet[i]
		for blockID := range blockIDStrSet {
			delete(va.pendingVotes.votes, blockID)
			delete(va.blockIDToVotingStatus, blockID)
			delete(va.createdQC, blockID)
			delete(va.proposerVotes, blockID)
		}
		delete(va.viewToBlockIDSet, i)
		delete(va.viewToVoteID, i)
	}
	va.highestPrunedView = view
}

// storeIncorporatedVote stores incorporated votes and accumulate stakes
// it drops invalid votes and duplicate votes
func (va *VoteAggregator) validateAndStoreIncorporatedVote(vote *hotstuff.Vote, block *hotstuff.Block) (bool, error) {
	voter, err := va.voteValidator.ValidateVote(vote, block)
	// does not report invalid vote as an error, notify consumers instead
	if err != nil {
		switch err := err.(type) {
		case hotstuff.InvalidVoteError:
			va.notifier.OnInvalidVoteDetected(vote)
			va.logger.Warn().Msg(err.Error())
			return false, nil
		default:
			return false, fmt.Errorf("could not validate incorporated vote: %w", err)
		}
	}
	// does not report double vote as an error, notify consumers instead
	firstVote, detected := va.detectDoubleVote(vote, voter)
	if detected {
		va.notifier.OnDoubleVotingDetected(firstVote, vote)
		return false, nil
	}
	// update existing voting status or create a new one
	votingStatus, exists := va.blockIDToVotingStatus[vote.BlockID]
	if !exists {
		threshold, err := va.viewState.GetQCStakeThresholdAtBlock(vote.BlockID)
		if err != nil {
			return false, fmt.Errorf("could not get stake threshold: %w", err)
		}
		identities, err := va.viewState.GetStakedIdentitiesAtBlock(vote.BlockID)
		if err != nil {
			return false, fmt.Errorf("could not get identities: %w", err)
		}
		votingStatus = NewVotingStatus(va.sigAggregator, threshold, vote.View, uint32(len(identities)), voter, vote.BlockID)
		va.blockIDToVotingStatus[vote.BlockID] = votingStatus
	}
	votingStatus.AddVote(vote, voter)
	va.updateState(vote)
	return true, nil
}

func (va *VoteAggregator) updateState(vote *hotstuff.Vote) {
	voterID := vote.Signature.SignerID

	// update viewToVoteID
	idToVote, exists := va.viewToVoteID[vote.View]
	if exists {
		idToVote[voterID] = vote
	} else {
		idToVote = make(map[flow.Identifier]*hotstuff.Vote)
		idToVote[voterID] = vote
		va.viewToVoteID[vote.View] = idToVote
	}

	// update viewToBlockIDSet
	blockIDSet, exists := va.viewToBlockIDSet[vote.View]
	if exists {
		blockIDSet[vote.BlockID] = struct{}{}
	} else {
		blockIDSet = make(map[flow.Identifier]struct{})
		blockIDSet[vote.BlockID] = struct{}{}
		va.viewToBlockIDSet[vote.View] = blockIDSet
	}
}

func (va *VoteAggregator) tryBuildQC(blockID flow.Identifier) (*hotstuff.QuorumCertificate, bool, error) {
	votingStatus, exists := va.blockIDToVotingStatus[blockID]
	if !exists { // can not build a qc if voting status doesn't exist
		return nil, false, nil
	}
	qc, built, err := votingStatus.TryBuildQC()
	if err != nil { // can not build a qc if there is an error
		return nil, false, err
	}
	if !built { // votes are insufficient
		return nil, false, nil
	}
	va.createdQC[votingStatus.blockID] = qc
	return qc, true, nil
}

// double voting is detected when the voter has voted a different block at the same view before
func (va *VoteAggregator) detectDoubleVote(vote *hotstuff.Vote, sender *flow.Identity) (*hotstuff.Vote, bool) {
	idToVotes, ok := va.viewToVoteID[vote.View]
	if !ok {
		// never voted by anyone
		return nil, false
	}
	originalVote, exists := idToVotes[sender.ID()]
	if !exists {
		// never voted by this sender
		return nil, false
	}
	if originalVote.BlockID == vote.BlockID {
		// voted and is the same vote as the vote received before
		return nil, false
	}
	return originalVote, true
}

func (va *VoteAggregator) canBuildQC(blockID flow.Identifier) bool {
	votingStatus, exists := va.blockIDToVotingStatus[blockID]
	if !exists {
		return false
	}
	return votingStatus.CanBuildQC()
}

func (va *VoteAggregator) isStale(vote *hotstuff.Vote) bool {
	return vote.View <= va.highestPrunedView
}

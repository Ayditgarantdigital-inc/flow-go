package verification

import (
	"fmt"

	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/protocol"
)

// SingleSigner is a signer capable of adding single signatures that can be
// aggregated to data structures.
type SingleSigner struct {
	*SingleVerifier
	signerID flow.Identifier
	signer   module.AggregatingSigner
}

// NewSingleSigner initializes a single signer with the given dependencies:
// - the given protocol state is used to retrieve public keys for the verifier;
// - the given signer is used to generate signatures for the local node; and
// - the given signer ID is used as identifier for our signatures.
func NewSingleSigner(state protocol.State, signer module.AggregatingSigner, signerID flow.Identifier) *SingleSigner {
	sc := &SingleSigner{
		SingleVerifier: NewSingleVerifier(state, signer),
		signerID:       signerID,
		signer:         signer,
	}
	return sc
}

// CreateProposal creates a proposal with a single signature for the given block.
func (s *SingleSigner) CreateProposal(block *model.Block) (*model.Proposal, error) {

	// check that the block is created by us
	if block.ProposerID != s.signerID {
		return nil, fmt.Errorf("can't create proposal for someone else's block")
	}

	// create the message to be signed and generate signature
	msg := messageFromParams(block.View, block.BlockID)
	sig, err := s.signer.Sign(msg)
	if err != nil {
		return nil, fmt.Errorf("could not generate staking signature: %w", err)
	}

	// create the proposal
	proposal := &model.Proposal{
		Block:   block,
		SigData: sig,
	}

	return proposal, nil
}

// CreateVote creates a vote with a single signature for the given block.
func (s *SingleSigner) CreateVote(block *model.Block) (*model.Vote, error) {

	// create the message to be signed and generate signature
	msg := messageFromParams(block.View, block.BlockID)
	sig, err := s.signer.Sign(msg)
	if err != nil {
		return nil, fmt.Errorf("could not generate staking signature: %w", err)
	}

	// create the vote
	vote := &model.Vote{
		View:     block.View,
		BlockID:  block.BlockID,
		SignerID: s.signerID,
		SigData:  sig,
	}

	return vote, nil
}

// CreateQC generates a quorum certificate with a single aggregated signature for the
// given votes.
func (s *SingleSigner) CreateQC(votes []*model.Vote) (*model.QuorumCertificate, error) {

	// check the consistency of the votes
	err := checkVotesValidity(votes)
	if err != nil {
		return nil, fmt.Errorf("votes are not valid: %w", err)
	}

	// collect all the vote signatures
	signerIDs := make([]flow.Identifier, len(votes))
	sigs := make([]crypto.Signature, 0, len(votes))
	for _, vote := range votes {
		signerIDs = append(signerIDs, vote.SignerID)
		sigs = append(sigs, vote.SigData)
	}

	// aggregate the signatures
	aggSig, err := s.signer.Aggregate(sigs)
	if err != nil {
		return nil, fmt.Errorf("could not aggregate signatures: %w", err)
	}

	// create the QC
	qc := &model.QuorumCertificate{
		View:      votes[0].View,
		BlockID:   votes[0].BlockID,
		SignerIDs: signerIDs,
		SigData:   aggSig,
	}

	return qc, nil
}

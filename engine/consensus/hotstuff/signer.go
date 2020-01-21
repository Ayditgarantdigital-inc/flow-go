package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

// Signer returns a signature for the given types
type Signer interface {
	SignVote(*types.UnsignedVote, uint32) *types.Signature
	SignBlockProposal(*types.UnsignedBlockProposal, uint32) *types.Signature
	// SignChallenge()
}

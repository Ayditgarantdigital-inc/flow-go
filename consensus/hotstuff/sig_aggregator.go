package hotstuff

import (
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
)

// SigAggregator aggregates signatures into an aggregated signature
type SigAggregator interface {

	// Aggregate aggregates the signatures into an aggregated signature.
	// It assumes:
	// 1. The given signatures are all valid.
	// 2. The signers of the signatures own enough stakes.
	// 3. The signatures contain enough shares to reconstruct the threshold signature
	Aggregate(block *model.Block, sigs []*model.SingleSignature) (*model.AggregatedSignature, error)

	// CanReconstruct checks whether the given sig shares are enough to reconstruct the threshold signature.
	// It assumes the DKG group size never change.
	CanReconstruct(numOfSigShares int) bool
}

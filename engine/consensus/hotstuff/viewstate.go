package hotstuff

import "github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"

type ViewState interface {
	IsSelf(id types.ID) bool
	IsSelfLeaderForView(view uint64) bool
	LeaderForView(view uint64) types.ID
}

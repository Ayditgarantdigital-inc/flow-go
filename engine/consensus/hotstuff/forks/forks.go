package forks

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/types"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Forks implements the hotstuff.Reactor API
type Forks struct {
	finalizer  Finalizer
	forkchoice ForkChoice
}

func New(finalizer Finalizer, forkchoice ForkChoice) *Forks {
	return &Forks{
		finalizer:  finalizer,
		forkchoice: forkchoice,
	}
}

func (f *Forks) GetBlocksForView(view uint64) []*types.Block {
	return f.finalizer.GetBlocksForView(view)
}

func (f *Forks) GetBlock(id flow.Identifier) (*types.Block, bool) {
	return f.finalizer.GetBlock(id)
}

func (f *Forks) FinalizedBlock() *types.Block {
	return f.finalizer.FinalizedBlock()
}

func (f *Forks) FinalizedView() uint64 {
	return f.FinalizedBlock().View
}

func (f *Forks) IsSafeBlock(block *types.Block) bool {
	if err := f.finalizer.VerifyBlock(block); err != nil {
		return false
	}
	return f.finalizer.IsSafeBlock(block)
}

func (f *Forks) AddBlock(block *types.Block) error {
	if err := f.finalizer.VerifyBlock(block); err != nil {
		// technically, this not strictly required. However, we leave this as a sanity check for now
		return fmt.Errorf("cannot add invalid block to Forks: %w", err)
	}
	err := f.finalizer.AddBlock(block)
	if err != nil {
		return fmt.Errorf("error storing block in Forks: %w", err)
	}

	// We only process the block's QC if the block's view is larger than the last finalized block.
	// By ignoring hte qc's of block's at or below the finalized view, we allow the genesis block
	// to have a nil QC.
	if block.View <= f.finalizer.FinalizedBlock().View {
		return nil
	}
	return f.AddQC(block.QC)
}

func (f *Forks) MakeForkChoice(curView uint64) (*types.Block, *types.QuorumCertificate, error) {
	return f.forkchoice.MakeForkChoice(curView)
}

func (f *Forks) AddQC(qc *types.QuorumCertificate) error {
	return f.forkchoice.AddQC(qc) // forkchoice ensures that block referenced by qc is known
}

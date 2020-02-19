package forkchoice

import (
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/engine/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/model/hotstuff"
)

// NewestForkChoice implements HotStuff's original fork choice rule:
// always use the newest QC (i.e. the QC with highest view number)
type NewestForkChoice struct {
	// preferredParent stores the preferred parent to build a block on top of. It contains the
	// parent block as well as the QC POINTING to the parent, which can be used to build the block.
	// preferredParent.QC called 'GenericQC' in 'Chained HotStuff Protocol' https://arxiv.org/abs/1803.05069v6
	preferredParent *forks.BlockQC
	finalizer       *finalizer.Finalizer
	notifier        notifications.Consumer
}

func NewNewestForkChoice(finalizer *finalizer.Finalizer, notifier notifications.Consumer) (*NewestForkChoice, error) {

	// build the initial block-QC pair
	// NOTE: I don't like this structure because it stores view and block ID in two separate places; this means
	// we don't have a single field that is the source of truth, and opens the door for bugs that would otherwise
	// be impossible
	block := finalizer.FinalizedBlock()
	qc := finalizer.FinalizedBlockQC()
	if block.BlockID != qc.BlockID || block.View != qc.View {
		return nil, fmt.Errorf("mismatch between finalized block and QC")
	}

	blockQC := forks.BlockQC{Block: block, QC: qc}

	fc := &NewestForkChoice{
		preferredParent: &blockQC,
		finalizer:       finalizer,
		notifier:        notifier,
	}

	notifier.OnQcIncorporated(qc)

	return fc, nil
}

// MakeForkChoice prompts the ForkChoice to generate a fork choice for the
// current view `curView`. NewestForkChoice always returns the qc with the largest
// view number seen.
//
// PREREQUISITE:
// ForkChoice cannot generate ForkChoices retroactively for past views.
// If used correctly, MakeForkChoice should only ever have processed QCs
// whose view is smaller than curView, for the following reason:
// Processing a QC with view v should result in the PaceMaker being in
// view v+1 or larger. Hence, given that the current View is curView,
// all QCs should have view < curView.
// To prevent accidental miss-usage, ForkChoices will error if `curView`
// is smaller than the view of any qc ForkChoice has seen.
// Note that tracking the view of the newest qc is for safety purposes
// and _independent_ of the fork-choice rule.
func (fc *NewestForkChoice) MakeForkChoice(curView uint64) (*hotstuff.Block, *hotstuff.QuorumCertificate, error) {
	choice := fc.preferredParent
	if choice.Block.View >= curView {
		// sanity check;
		// processing a QC with view v should result in the PaceMaker being in view v+1 or larger
		// Hence, given that the current View is curView, all QCs should have view < curView
		return nil, nil, fmt.Errorf(
			"ForkChoice selected qc with view %d which is larger than requested view %d",
			choice.Block.View, curView,
		)
	}
	fc.notifier.OnForkChoiceGenerated(curView, choice.QC)
	return choice.Block, choice.QC, nil
}

// updateQC updates `preferredParent` according to the fork-choice rule.
// Currently, we implement 'Chained HotStuff Protocol' where the fork-choice
// rule is: "build on newest QC"
func (fc *NewestForkChoice) AddQC(qc *hotstuff.QuorumCertificate) error {
	if qc.View <= fc.preferredParent.Block.View {
		// Per construction, preferredParent.View() is always larger than the last finalized block's view.
		// Hence, this check suffices to drop all QCs with qc.View <= last_finalized_block.View().
		return nil
	}

	// Have qc.View > last_finalized_block.View(). Hence, block referenced by qc should be stored:
	block, err := fc.ensureBlockStored(qc)
	if err != nil {
		return fmt.Errorf("cannot add QC: %w", err)
	}

	if block.BlockID != qc.BlockID || block.View != qc.View {
		return fmt.Errorf("mismatch between finalized block and QC")
	}

	blockQC := forks.BlockQC{Block: block, QC: qc}
	fc.preferredParent = &blockQC
	fc.notifier.OnQcIncorporated(qc)

	return nil
}

func (fc *NewestForkChoice) ensureBlockStored(qc *hotstuff.QuorumCertificate) (*hotstuff.Block, error) {
	block, haveBlock := fc.finalizer.GetBlock(qc.BlockID)
	if !haveBlock {
		return nil, &hotstuff.ErrorMissingBlock{View: qc.View, BlockID: qc.BlockID}
	}
	if block.View != qc.View {
		return nil, fmt.Errorf("invalid qc with mismatching view")
	}
	return block, nil
}

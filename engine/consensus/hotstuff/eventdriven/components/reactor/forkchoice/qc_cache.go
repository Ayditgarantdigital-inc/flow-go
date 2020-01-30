package forkchoice

import (
	"bytes"
	"fmt"

	"github.com/dapperlabs/flow-go/engine/consensus/eventdriven/modules/def"
)

type QcSet map[string]*def.QuorumCertificate

type QcCache struct {
	qcAtView   map[uint64]*def.QuorumCertificate
	LowestView uint64
}

// NewQcCache initializes a QcCache
func NewQcCache() QcCache {
	return QcCache{
		qcAtView: make(map[uint64]*def.QuorumCertificate),
	}
}

// pruneAtView prunes all blocks up to and INCLUDING `level`
func (cache *QcCache) PruneAtView(view uint64) {
	if view+1 < cache.LowestView {
		panic(fmt.Sprintf("Cannot cache up to level %d because we only save up to level %d", view, cache.LowestView))
	}
	for v := cache.LowestView; v <= view; v++ {
		delete(cache.qcAtView, v)
	}
	cache.LowestView = view + 1
}

// AddQC adds the QC to the Cache
// Safe:
// * Gracefully handles repeated addition of QCs for same block (keeps first added QC)
// * if QC is at or below pruning view: NoOp
// * checks for inconsistencies:
//   if QC for same Block-Merkle-Root-Hash (blockMRH) but different view exists => panic
//   (instead of leaving the data structure in an inconsistent state).
func (cache *QcCache) AddQC(qc *def.QuorumCertificate) {
	if qc.View < cache.LowestView {
		return
	}
	otherQc, exists := cache.qcAtView[qc.View]
	if !exists {
		cache.qcAtView[qc.View] = qc
	}
	if !bytes.Equal(qc.BlockMRH, otherQc.BlockMRH) {
		// QC for block at same view exists but is for different block
		// This means we have at least 1/3 byzantine actors. We are done.
		panic("Encountered QCs for conflicting blocks at same View.")
	}
}

// GetQC returns (<QuorumCertificate>, true) if the QC for given view was found
// and (nil, false) otherwise
func (cache *QcCache) GetQC(view uint64) (*def.QuorumCertificate, bool) {
	if view < cache.LowestView {
		return nil, false
	}
	qc, exists := cache.qcAtView[view]
	if !exists {
		return nil, false
	}
	return qc, true
}

// GetQCForBlock returns (<QuorumCertificate>, true) if the QC for given block was found
// and (nil, false) otherwise
func (cache *QcCache) GetQCForBlock(blockMRH []byte, blockView uint64) (*def.QuorumCertificate, bool) {
	if blockView < cache.LowestView {
		return nil, false
	}
	qc, exists := cache.qcAtView[blockView]
	if !exists || !bytes.Equal(blockMRH, qc.BlockMRH) {
		return nil, false
	}
	return qc, true
}

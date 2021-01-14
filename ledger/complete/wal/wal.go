package wal

import (
	"fmt"
	"sort"

	prometheusWAL "github.com/m4ksio/wal/wal"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/complete/mtrie"
	"github.com/onflow/flow-go/ledger/complete/mtrie/flattener"
)

const SegmentSize = 32 * 1024 * 1024

type LedgerWAL struct {
	wal            *prometheusWAL.WAL
	paused         bool
	forestCapacity int
	pathByteSize   int
	log            zerolog.Logger
}

// TODO use real logger and metrics, but that would require passing them to Trie storage
func NewWAL(logger zerolog.Logger, reg prometheus.Registerer, dir string, forestCapacity int, pathByteSize int, segmentSize int) (*LedgerWAL, error) {
	w, err := prometheusWAL.NewSize(logger, reg, dir, segmentSize, false)
	if err != nil {
		return nil, err
	}
	return &LedgerWAL{
		wal:            w,
		paused:         false,
		forestCapacity: forestCapacity,
		pathByteSize:   pathByteSize,
		log:            logger,
	}, nil
}

func (w *LedgerWAL) PauseRecord() {
	w.paused = true
}

func (w *LedgerWAL) UnpauseRecord() {
	w.paused = false
}

func (w *LedgerWAL) RecordUpdate(update *ledger.TrieUpdate) error {
	if w.paused {
		return nil
	}

	bytes := EncodeUpdate(update)

	_, err := w.wal.Log(bytes)

	if err != nil {
		return fmt.Errorf("error while recording update in LedgerWAL: %w", err)
	}
	return nil
}

func (w *LedgerWAL) RecordDelete(rootHash ledger.RootHash) error {
	if w.paused {
		return nil
	}

	bytes := EncodeDelete(rootHash)

	_, err := w.wal.Log(bytes)

	if err != nil {
		return fmt.Errorf("error while recording delete in LedgerWAL: %w", err)
	}
	return nil
}

func (w *LedgerWAL) ReplayOnForest(forest *mtrie.Forest) error {
	return w.Replay(
		func(forestSequencing *flattener.FlattenedForest) error {
			rebuiltTries, err := flattener.RebuildTries(forestSequencing)
			if err != nil {
				return fmt.Errorf("rebuilding forest from sequenced nodes failed: %w", err)
			}
			err = forest.AddTries(rebuiltTries)
			if err != nil {
				return fmt.Errorf("adding rebuilt tries to forest failed: %w", err)
			}
			return nil
		},
		func(update *ledger.TrieUpdate) error {
			_, err := forest.Update(update)
			return err
		},
		func(rootHash ledger.RootHash) error {
			forest.RemoveTrie(rootHash)
			return nil
		},
	)
}

func (w *LedgerWAL) Segments() (first, last int, err error) {
	return prometheusWAL.Segments(w.wal.Dir())
}

func (w *LedgerWAL) Replay(
	checkpointFn func(forestSequencing *flattener.FlattenedForest) error,
	updateFn func(update *ledger.TrieUpdate) error,
	deleteFn func(ledger.RootHash) error,
) error {
	from, to, err := w.Segments()
	if err != nil {
		return err
	}
	return w.replay(from, to, checkpointFn, updateFn, deleteFn, true)
}

func (w *LedgerWAL) ReplayLogsOnly(
	checkpointFn func(forestSequencing *flattener.FlattenedForest) error,
	updateFn func(update *ledger.TrieUpdate) error,
	deleteFn func(rootHash ledger.RootHash) error,
) error {
	from, to, err := w.Segments()
	if err != nil {
		return err
	}
	return w.replay(from, to, checkpointFn, updateFn, deleteFn, false)
}

func (w *LedgerWAL) replay(
	from, to int,
	checkpointFn func(forestSequencing *flattener.FlattenedForest) error,
	updateFn func(update *ledger.TrieUpdate) error,
	deleteFn func(rootHash ledger.RootHash) error,
	useCheckpoints bool,
) error {

	if to < from {
		return fmt.Errorf("end of range cannot be smaller than beginning")
	}

	loadedCheckpoint := -1
	startSegment := from

	checkpointer, err := w.NewCheckpointer()
	if err != nil {
		return fmt.Errorf("cannot create checkpointer: %w", err)
	}

	if useCheckpoints {
		allCheckpoints, err := checkpointer.ListCheckpoints()
		if err != nil {
			return fmt.Errorf("cannot get list of checkpoints: %w", err)
		}

		var availableCheckpoints []int

		// if there are no checkpoints already, don't bother
		if len(allCheckpoints) > 0 {
			// from-1 to account for checkpoints connected to segments, ie. checkpoint 8 if replaying segments 9-12
			availableCheckpoints = getPossibleCheckpoints(allCheckpoints, from-1, to)
		}

		for len(availableCheckpoints) > 0 { // as long as there are checkpoints to try
			latestCheckpoint := availableCheckpoints[len(availableCheckpoints)-1]

			forestSequencing, err := checkpointer.LoadCheckpoint(latestCheckpoint)
			if err != nil {
				w.log.Warn().Int("checkpoint", latestCheckpoint).Err(err).
					Msg("checkpoint loading failed")

				availableCheckpoints = availableCheckpoints[:len(availableCheckpoints)-1]
				continue
			}
			err = checkpointFn(forestSequencing)
			if err != nil {
				return fmt.Errorf("error while handling checkpoint: %w", err)
			}
			loadedCheckpoint = latestCheckpoint
			break
		}

		if loadedCheckpoint != -1 && to == loadedCheckpoint {
			return nil
		}

		if loadedCheckpoint >= 0 {
			startSegment = loadedCheckpoint + 1
		}
	}

	if loadedCheckpoint == -1 && startSegment == 0 {
		hasRootCheckpoint, err := checkpointer.HasRootCheckpoint()
		if err != nil {
			return fmt.Errorf("cannot check root checkpoint existence: %w", err)
		}
		if hasRootCheckpoint {
			flattenedForest, err := checkpointer.LoadRootCheckpoint()
			if err != nil {
				return fmt.Errorf("cannot load root checkpoint: %w", err)
			}
			err = checkpointFn(flattenedForest)
			if err != nil {
				return fmt.Errorf("error while handling root checkpoint: %w", err)
			}
		}
	}

	sr, err := prometheusWAL.NewSegmentsRangeReader(prometheusWAL.SegmentRange{
		Dir:   w.wal.Dir(),
		First: startSegment,
		Last:  to,
	})
	if err != nil {
		return fmt.Errorf("cannot create segment reader: %w", err)
	}

	reader := prometheusWAL.NewReader(sr)

	defer sr.Close()

	for reader.Next() {
		record := reader.Record()
		operation, rootHash, update, err := Decode(record)
		if err != nil {
			return fmt.Errorf("cannot decode LedgerWAL record: %w", err)
		}

		switch operation {
		case WALUpdate:
			err = updateFn(update)
			if err != nil {
				return fmt.Errorf("error while processing LedgerWAL update: %w", err)
			}
		case WALDelete:
			err = deleteFn(rootHash)
			if err != nil {
				return fmt.Errorf("error while processing LedgerWAL deletion: %w", err)
			}
		}

		err = reader.Err()
		if err != nil {
			return fmt.Errorf("cannot read LedgerWAL: %w", err)
		}
	}
	return nil
}

func getPossibleCheckpoints(allCheckpoints []int, from, to int) []int {
	//list of checkpoints is sorted
	indexFrom := sort.SearchInts(allCheckpoints, from)
	indexTo := sort.SearchInts(allCheckpoints, to)

	// all checkpoints are earlier, return last one
	if indexTo == len(allCheckpoints) {
		return allCheckpoints[indexFrom:indexTo]
	}

	// exact match
	if allCheckpoints[indexTo] == to {
		return allCheckpoints[indexFrom : indexTo+1]
	}

	// earliest checkpoint from list doesn't match, index 0 means no match at all
	if indexTo == 0 {
		return nil
	}

	return allCheckpoints[indexFrom:indexTo]
}

// NewCheckpointer returns a Checkpointer for this WAL
func (w *LedgerWAL) NewCheckpointer() (*Checkpointer, error) {
	return NewCheckpointer(w, w.pathByteSize, w.forestCapacity), nil
}

func (w *LedgerWAL) Close() error {
	return w.wal.Close()
}

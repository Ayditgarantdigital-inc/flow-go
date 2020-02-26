package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Snapshot represents a snapshot of chain state anchored at a particular
// reference block.
//
// If final is true, the reference is the latest finalized block. If final is
// false and blockID is set, the reference is the block with the given ID.
type Snapshot struct {
	state   *State
	blockID flow.Identifier
	final   bool
}

func (s *Snapshot) Collection() (*flow.LightCollection, error) {
	var collection flow.LightCollection

	err := s.state.db.View(func(tx *badger.Txn) error {

		// get the header for this snapshot
		var header flow.Header
		err := s.head(&header)(tx)
		if err != nil {
			return fmt.Errorf("failed to get snapshot header: %w", err)
		}

		// get the payload
		var payload cluster.Payload
		err = procedure.RetrieveClusterPayload(header.PayloadHash, &payload)(tx)
		if err != nil {
			return fmt.Errorf("failed to get snapshot payload: %w", err)
		}

		// set the collection
		collection = payload.Collection

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &collection, nil
}

func (s *Snapshot) Head() (*flow.Header, error) {
	var head flow.Header
	err := s.state.db.View(func(tx *badger.Txn) error {
		return s.head(&head)(tx)
	})
	return &head, err
}

// head finds the header referenced by the snapshot.
func (s *Snapshot) head(head *flow.Header) func(*badger.Txn) error {
	return func(tx *badger.Txn) error {

		// if final is set and block ID is empty, set block ID to last finalized
		if s.final && s.blockID == flow.ZeroID {

			// get the boundary
			var boundary uint64
			err := operation.RetrieveBoundaryForCluster(s.state.chainID, &boundary)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve boundary: %w", err)
			}

			// get the ID of the last finalized block
			err = operation.RetrieveNumberForCluster(s.state.chainID, boundary, &s.blockID)(tx)
			if err != nil {
				return fmt.Errorf("could not retrieve block ID: %w", err)
			}
		}

		// get the snapshot header
		err := operation.RetrieveHeader(s.blockID, head)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve header for block (%x): %w", s.blockID, err)
		}

		return nil
	}
}

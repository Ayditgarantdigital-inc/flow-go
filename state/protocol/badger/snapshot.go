// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package badger

import (
	"errors"
	"fmt"
	"sort"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/model/flow/order"
	"github.com/dapperlabs/flow-go/state"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

// Snapshot represents a read-only immutable snapshot of the protocol state.
type Snapshot struct {
	err     error
	state   *State
	blockID flow.Identifier
}

// Identities retrieves all active ids at the given snapshot and
// applies the given filters.
func (s *Snapshot) Identities(selector flow.IdentityFilter) (flow.IdentityList, error) {
	if s.err != nil {
		return nil, s.err
	}

	// retrieve the root height
	var height uint64
	err := s.state.db.View(operation.RetrieveRootHeight(&height))
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root height: %w", err)
	}

	// retrieve root block ID
	var rootID flow.Identifier
	err = s.state.db.View(operation.LookupBlockHeight(height, &rootID))
	if err != nil {
		return nil, fmt.Errorf("could not look up root block: %w", err)
	}

	// retrieve identities from storage
	payload, err := s.state.payloads.ByBlockID(rootID)
	if err != nil {
		return nil, fmt.Errorf("could not get root block payload: %w", err)
	}

	// apply the filter to the identities
	identities := payload.Identities.Filter(selector)

	// apply a deterministic sort to the identities
	sort.Slice(identities, func(i int, j int) bool {
		return order.ByNodeIDAsc(identities[i], identities[j])
	})

	return identities, err
}

func (s *Snapshot) Identity(nodeID flow.Identifier) (*flow.Identity, error) {
	if s.err != nil {
		return nil, s.err
	}

	// filter identities at snapshot for node ID
	identities, err := s.Identities(filter.HasNodeID(nodeID))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	// check if node ID is part of identities
	if len(identities) == 0 {
		return nil, protocol.IdentityNotFoundErr{
			NodeID: nodeID,
		}
	}

	return identities[0], nil
}

func (s *Snapshot) Commit() (flow.StateCommitment, error) {
	if s.err != nil {
		return nil, s.err
	}

	// get the ID of the sealed block
	seal, err := s.state.seals.ByBlockID(s.blockID)
	if err != nil {
		return nil, fmt.Errorf("could not get look up sealed commit: %w", err)
	}

	return seal.FinalState, nil
}

// Clusters sorts the list of node identities after filtering into the given
// number of clusters.
//
// This is guaranteed to be deterministic for an identical set of identities,
// regardless of the order.
func (s *Snapshot) Clusters() (*flow.ClusterList, error) {
	if s.err != nil {
		return nil, s.err
	}

	// get the node identities
	identities, err := s.Identities(filter.HasRole(flow.RoleCollection))
	if err != nil {
		return nil, fmt.Errorf("could not get identities: %w", err)
	}

	return protocol.Clusters(s.state.clusters, identities), nil
}

func (s *Snapshot) Head() (*flow.Header, error) {
	if s.err != nil {
		return nil, s.err
	}

	return s.state.headers.ByBlockID(s.blockID)
}

func (s *Snapshot) Seed(indices ...uint32) ([]byte, error) {
	if s.err != nil {
		return nil, s.err
	}

	// get the current state snapshot head
	var childrenIDs []flow.Identifier
	err := s.state.db.View(procedure.LookupBlockChildren(s.blockID, &childrenIDs))
	if err != nil {
		return nil, fmt.Errorf("could not look up children: %w", err)
	}

	// check we have at least one child
	if len(childrenIDs) == 0 {
		return nil, state.NewNoValidChildBlockError("block doesn't have children yet")
	}

	// find the first child that has been validated
	var validChildID flow.Identifier
	for _, childID := range childrenIDs {
		var valid bool
		err = s.state.db.View(operation.RetrieveBlockValidity(childID, &valid))
		// skip blocks whose validity hasn't been checked yet
		if errors.Is(err, storage.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("could not get child validity: %w", err)
		}
		if valid {
			validChildID = childID
			break
		}
	}

	if validChildID == flow.ZeroID {
		return nil, state.NewNoValidChildBlockError("block has no valid children")
	}

	// get the header of the first child (they all have the same threshold sig)
	head, err := s.state.headers.ByBlockID(validChildID)
	if err != nil {
		return nil, fmt.Errorf("could not get head: %w", err)
	}

	seed, err := protocol.SeedFromParentSignature(indices, head.ParentVoterSig)
	if err != nil {
		return nil, fmt.Errorf("could not create seed from header's signature: %w", err)
	}

	return seed, nil
}

func (s *Snapshot) Pending() ([]flow.Identifier, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.pending(s.blockID)
}

func (s *Snapshot) pending(blockID flow.Identifier) ([]flow.Identifier, error) {

	var pendingIDs []flow.Identifier
	err := s.state.db.View(procedure.LookupBlockChildren(blockID, &pendingIDs))
	if err != nil {
		return nil, fmt.Errorf("could not get pending children: %w", err)
	}

	for _, pendingID := range pendingIDs {
		additionalIDs, err := s.pending(pendingID)
		if err != nil {
			return nil, fmt.Errorf("could not get pending grandchildren: %w", err)
		}
		pendingIDs = append(pendingIDs, additionalIDs...)
	}
	return pendingIDs, nil
}

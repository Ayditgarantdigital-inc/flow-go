package state

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

// TODO Many operations here are should be transactional, so we need to refactor this
// to store a reference to DB and compose operations and procedures rather then
// just being amalgamate of proxies for single transactions operation

// ExecutionState is an interface used to access and mutate the execution state of the blockchain.
type ExecutionState interface {
	// NewView creates a new ready-only view at the given state commitment.
	NewView(flow.StateCommitment) *View
	// CommitDelta commits a register delta and returns the new state commitment.
	CommitDelta(Delta) (flow.StateCommitment, error)

	GetRegisters(flow.StateCommitment, []flow.RegisterID) ([]flow.RegisterValue, error)
	GetChunkRegisters(flow.Identifier) (flow.Ledger, error)

	// StateCommitmentByBlockID returns the final state commitment for the provided block ID.
	StateCommitmentByBlockID(flow.Identifier) (flow.StateCommitment, error)
	// PersistStateCommitment saves a state commitment by the given block ID.
	PersistStateCommitment(flow.Identifier, flow.StateCommitment) error

	// ChunkHeaderByChunkID returns the chunk header for the provided chunk ID.
	ChunkHeaderByChunkID(flow.Identifier) (*flow.ChunkHeader, error)
	// PersistChunkHeader saves a chunk header by chunk ID.
	PersistChunkHeader(*flow.ChunkHeader) error

	GetExecutionResultID(blockID flow.Identifier) (flow.Identifier, error)
	PersistExecutionResult(blockID flow.Identifier, result flow.ExecutionResult) error
}

type state struct {
	ls               storage.Ledger
	commits          storage.Commits
	chunkHeaders     storage.ChunkHeaders
	executionResults storage.ExecutionResults
}

// NewExecutionState returns a new execution state access layer for the given ledger storage.
func NewExecutionState(
	ls storage.Ledger,
	commits storage.Commits,
	chunkHeaders storage.ChunkHeaders,
	executionResult storage.ExecutionResults,
) ExecutionState {
	return &state{
		ls:               ls,
		commits:          commits,
		chunkHeaders:     chunkHeaders,
		executionResults: executionResult,
	}
}

func (s *state) NewView(commitment flow.StateCommitment) *View {
	return NewView(func(key flow.RegisterID) ([]byte, error) {
		values, err := s.ls.GetRegisters(
			[]flow.RegisterID{key},
			commitment,
		)
		if err != nil {
			return nil, fmt.Errorf("error getting register (%s) value at %x: %w", key, commitment, err)
		}

		if len(values) == 0 {
			return nil, nil
		}

		return values[0], nil
	})
}

func (s *state) CommitDelta(delta Delta) (flow.StateCommitment, error) {
	ids, values := delta.RegisterUpdates()

	// TODO: update CommitDelta to also return proofs
	commit, _, err := s.ls.UpdateRegistersWithProof(ids, values)
	if err != nil {
		return nil, err
	}

	return commit, nil
}

func (s *state) GetRegisters(
	commit flow.StateCommitment,
	registerIDs []flow.RegisterID,
) ([]flow.RegisterValue, error) {
	return s.ls.GetRegisters(registerIDs, commit)
}

func (s *state) GetChunkRegisters(chunkID flow.Identifier) (flow.Ledger, error) {
	chunkHeader, err := s.ChunkHeaderByChunkID(chunkID)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve chunk header: %w", err)
	}

	registerIDs := chunkHeader.RegisterIDs

	registerValues, err := s.GetRegisters(chunkHeader.StartState, registerIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve chunk register values: %w", err)
	}

	l := make(flow.Ledger)

	for i, registerID := range registerIDs {
		l[string(registerID)] = registerValues[i]
	}

	return l, nil
}

func (s *state) StateCommitmentByBlockID(blockID flow.Identifier) (flow.StateCommitment, error) {
	return s.commits.ByID(blockID)
}

func (s *state) PersistStateCommitment(blockID flow.Identifier, commit flow.StateCommitment) error {
	return s.commits.Store(blockID, commit)
}

func (s *state) ChunkHeaderByChunkID(chunkID flow.Identifier) (*flow.ChunkHeader, error) {
	return s.chunkHeaders.ByID(chunkID)
}

func (s *state) PersistChunkHeader(c *flow.ChunkHeader) error {
	return s.chunkHeaders.Store(c)
}

func (s *state) GetExecutionResultID(blockID flow.Identifier) (flow.Identifier, error) {
	return s.executionResults.Lookup(blockID)
}

func (s *state) PersistExecutionResult(blockID flow.Identifier, result flow.ExecutionResult) error {
	err := s.executionResults.Store(&result)
	if err != nil {
		return fmt.Errorf("could not persist execution result: %w", err)
	}
	// TODO if the second operation fails we should remove stored execution result
	// This is global execution storage problem - see TODO at the top
	return s.executionResults.Index(blockID, result.ID())
}

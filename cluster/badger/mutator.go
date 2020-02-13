package badger

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
	"github.com/dapperlabs/flow-go/storage/badger/procedure"
)

type Mutator struct {
	state *State
}

func (m *Mutator) Bootstrap(genesis *cluster.Block) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// check header number
		if genesis.Number != 0 {
			return fmt.Errorf("genesis number should be 0 (got %d)", genesis.Number)
		}

		// check header parent ID
		if genesis.ParentID != flow.ZeroID {
			return fmt.Errorf("genesis parent ID must be zero hash (got %x)", genesis.ParentID)
		}

		// check payload
		collSize := len(genesis.Collection.Transactions)
		if collSize != 0 {
			return fmt.Errorf("genesis collection should contain no transactions (got %d)", collSize)
		}

		// check payload hash
		if genesis.PayloadHash != genesis.Payload.Hash() {
			return fmt.Errorf("genesis payload hash must match payload")
		}

		// insert block payload
		err := operation.InsertCollection(&genesis.Payload.Collection)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis block payload: %w", err)
		}

		// insert block
		err = procedure.InsertClusterBlock(genesis)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis block: %w", err)
		}

		// insert block number -> ID mapping
		err = operation.InsertNumberForCluster(genesis.ChainID, genesis.Number, genesis.ID())(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis number: %w", err)
		}

		// insert boundary
		err = operation.InsertBoundaryForCluster(genesis.ChainID, genesis.Number)(tx)
		if err != nil {
			return fmt.Errorf("could not insert genesis boundary: %w", err)
		}

		return nil
	})
}

func (m *Mutator) Extend(blockID flow.Identifier) error {
	return m.state.db.Update(func(tx *badger.Txn) error {

		// retrieve the block
		var block cluster.Block
		err := procedure.RetrieveClusterBlock(blockID, &block)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve block: %w", err)
		}

		// check block integrity
		if block.Payload.Hash() != block.PayloadHash {
			return fmt.Errorf("block payload does not match payload hash")
		}

		// get the chain ID, which determines which cluster state to query
		chainID := block.ChainID

		// get finalized state boundary
		var boundary uint64
		err = operation.RetrieveBoundaryForCluster(chainID, &boundary)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve boundary: %w", err)
		}

		// get the hash of the latest finalized block
		var lastFinalizedBlockID flow.Identifier
		err = operation.RetrieveNumberForCluster(chainID, boundary, &lastFinalizedBlockID)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve latest finalized ID: %w", err)
		}

		// get the header of the parent of the new block
		var parent flow.Header
		err = operation.RetrieveHeader(block.ParentID, &parent)(tx)
		if err != nil {
			return fmt.Errorf("could not retrieve latest finalized header: %w", err)
		}

		// if the new block has a lower number than its parent, we can't add it
		if block.Number <= parent.Number {
			return fmt.Errorf("extending block height (%d) must be > parent height (%d)", block.Number, parent.Number)
		}

		// trace back from new block until we find a block that has the latest
		// finalized block as its parent
		for block.ParentID != lastFinalizedBlockID {

			// get the parent of current block
			err = operation.RetrieveHeader(block.ParentID, &block.Header)(tx)
			if err != nil {
				return fmt.Errorf("could not get parent (%x): %w", block.ParentID, err)
			}

			// if its number is below current boundary, the block does not connect
			// to the finalized protocol state and would break database consistency
			if block.Number < boundary {
				return fmt.Errorf("block doesn't connect to finalized state")
			}
		}

		return nil
	})
}

package bootstrap

import (
	"encoding/hex"
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/engine/execution/computation/virtualmachine"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/execution/state/delta"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/storage/badger/operation"
)

// BootstrapLedger adds the above root account to the ledger and initializes execution node-only data
func BootstrapLedger(ledger storage.Ledger) (flow.StateCommitment, error) {
	view := delta.NewView(state.LedgerGetRegister(ledger, ledger.EmptyStateCommitment()))

	BootstrapView(view)

	newStateCommitment, err := state.CommitDelta(ledger, view.Delta(), ledger.EmptyStateCommitment())
	if err != nil {
		return nil, err
	}

	return newStateCommitment, nil
}

func BootstrapExecutionDatabase(db *badger.DB, genesis *flow.Header) error {
	err := db.Update(func(txn *badger.Txn) error {
		err := operation.InsertHighestExecutedBlockNumber(genesis.Height, genesis.ID())(txn)
		if err != nil {
			return err
		}

		views := make([]*delta.Snapshot, 0)
		err = operation.InsertExecutionStateInteractions(genesis.ID(), views)(txn)
		if err != nil {
			return err
		}

		return operation.IndexStateCommitment(flow.GenesisParentID, flow.GenesisStateCommitment)(txn)
	})

	if err != nil {
		return err
	}

	return nil
}

func BootstrapView(view *delta.View) {
	privateKeyBytes, err := hex.DecodeString(flow.RootAccountPrivateKeyHex)
	if err != nil {
		panic("Cannot hex decode hardcoded key!")
	}

	privateKey, err := flow.DecodeAccountPrivateKey(privateKeyBytes)
	if err != nil {
		panic("Cannot decode hardcoded private key!")
	}

	publicKeyBytes, err := flow.EncodeAccountPublicKey(privateKey.PublicKey(1000))
	if err != nil {
		panic("Cannot encode public key of hardcoded private key!")
	}
	_, err = virtualmachine.CreateAccountInLedger(view, [][]byte{publicKeyBytes})
	if err != nil {
		panic(fmt.Sprintf("error while creating account in ledger: %s ", err))
	}
}

package bootstrap

import (
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

func BootstrapExecutionDatabase(db *badger.DB, commit flow.StateCommitment, genesis *flow.Header) error {
	err := operation.RetryOnConflict(db.Update, func(txn *badger.Txn) error {

		err := operation.InsertExecutedBlock(genesis.ID())(txn)
		if err != nil {
			return fmt.Errorf("could not index initial genesis execution block: %w", err)
		}

		err = operation.IndexStateCommitment(flow.ZeroID, commit)(txn)
		if err != nil {
			return fmt.Errorf("could not index void state commitment: %w", err)
		}

		err = operation.IndexStateCommitment(genesis.ID(), commit)(txn)
		if err != nil {
			return fmt.Errorf("could not index genesis state commitment: %w", err)
		}

		views := make([]*delta.Snapshot, 0)
		err = operation.InsertExecutionStateInteractions(genesis.ID(), views)(txn)
		if err != nil {
			return fmt.Errorf("could not bootstrap execution state interactions: %w", err)
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

func BootstrapView(view *delta.View) {
	ledgerAccess := virtualmachine.LedgerDAL{Ledger: view}
	_, err := ledgerAccess.CreateAccountInLedger([]flow.AccountPublicKey{flow.RootAccountPrivateKey.PublicKey(1000)})
	if err != nil {
		panic(fmt.Sprintf("error while creating account in ledger: %s ", err))
	}
}

func deployFungibleToken(vm virtualmachine.VirtualMachine, view *delta.View) (flow.Address, error) {
	vm.NewContext(view).Invoke("")
	return flow.Address{}, nil
}

func deployFlowToken(vm virtualmachine.VirtualMachine, view *delta.View) (flow.Address, error) {
	return flow.Address{}, nil
}

func deployFeeContract(vm virtualmachine.VirtualMachine, view *delta.View) (flow.Address, error) {
	return flow.Address{}, nil
}

func createServiceAccount(vm virtualmachine.VirtualMachine, view *delta.View, fungibleToken, flowToken, feeContract flow.Address) (flow.Address, error) {
	return flow.Address{}, nil
}

func deployContract(vm virtualmachine.VirtualMachine, view *delta.View) error { return nil }

package emulator

import (
	"fmt"

	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

// ErrBlockNotFound indicates that a block specified by hash or number cannot be found.
type ErrBlockNotFound struct {
	BlockHash crypto.Hash
	BlockNum  uint64
}

func (e *ErrBlockNotFound) Error() string {
	if e.BlockNum == 0 {
		return fmt.Sprintf("Block with hash %s cannot be found", e.BlockHash)
	}

	return fmt.Sprintf("Block number %d cannot be found", e.BlockNum)
}

// ErrTransactionNotFound indicates that a transaction specified by hash cannot be found.
type ErrTransactionNotFound struct {
	TxHash crypto.Hash
}

func (e *ErrTransactionNotFound) Error() string {
	return fmt.Sprintf("Transaction with hash %s cannot be found", e.TxHash)
}

// ErrAccountNotFound indicates that an account specified by address cannot be found.
type ErrAccountNotFound struct {
	Address types.Address
}

func (e *ErrAccountNotFound) Error() string {
	return fmt.Sprintf("Account with address %s cannot be found", e.Address)
}

// ErrDuplicateTransaction indicates that a transaction has already been submitted.
type ErrDuplicateTransaction struct {
	TxHash crypto.Hash
}

func (e *ErrDuplicateTransaction) Error() string {
	return fmt.Sprintf("Transaction with hash %s has already been submitted", e.TxHash)
}

// ErrInvalidSignaturePublicKey indicates that signature uses an invalid public key.
type ErrInvalidSignaturePublicKey struct {
	Account types.Address
}

func (e *ErrInvalidSignaturePublicKey) Error() string {
	return fmt.Sprintf("Public key used for signing does not exist on account %s", e.Account)
}

// ErrInvalidSignatureAccount indicates that a signature references a nonexistent account.
type ErrInvalidSignatureAccount struct {
	Account types.Address
}

func (e *ErrInvalidSignatureAccount) Error() string {
	return fmt.Sprintf("Account with address %s does not exist", e.Account)
}

// ErrTransactionReverted indicates that a transaction reverted.
type ErrTransactionReverted struct {
	TxHash crypto.Hash
	Err    error
}

func (e *ErrTransactionReverted) Error() string {
	return fmt.Sprintf(
		"Transaction with hash %s reverted during execution: %s",
		e.TxHash,
		e.Err.Error(),
	)
}

// ErrInvalidStateVersion indicates that a state version hash provided is invalid.
type ErrInvalidStateVersion struct {
	Version crypto.Hash
}

func (e *ErrInvalidStateVersion) Error() string {
	return fmt.Sprintf("World State with version hash %s is invalid", e.Version)
}

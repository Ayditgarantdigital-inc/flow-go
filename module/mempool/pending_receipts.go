package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification"
)

// PendingReceipts represents a concurrency-safe memory pool for pending execution receipts.
type PendingReceipts interface {

	// Add will add the given pending receipt to the memory pool. It will return
	// false if it was already in the mempool.
	Add(preceipt *verification.PendingReceipt) bool

	// Has checks if the given receipt is part of the memory pool.
	Has(preceiptID flow.Identifier) bool

	// Rem will remove a receipt by ID.
	Rem(preceiptID flow.Identifier) bool

	// All will return a list of all receipts in the memory pool.
	All() []*verification.PendingReceipt
}

// Package cluster contains models related to collection node cluster
// consensus.
package cluster

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

func Genesis() *Block {
	header := flow.Header{
		View:      0,
		ChainID:   "",
		Timestamp: flow.GenesisTime(),
		ParentID:  flow.ZeroID,
	}

	payload := EmptyPayload()

	header.PayloadHash = payload.Hash()

	block := &Block{
		Header:  header,
		Payload: payload,
	}

	return block
}

// Block represents a block in collection node cluster consensus. It contains
// a standard block header with a payload containing only a single collection.
type Block struct {
	flow.Header
	Payload
}

// SetPayload sets the payload and payload hash.
func (b *Block) SetPayload(payload Payload) {
	b.Payload = payload
	b.PayloadHash = payload.Hash()
}

// Payload is the payload for blocks in collection node cluster consensus.
// It contains only a single collection.
type Payload struct {
	Collection flow.Collection
}

func EmptyPayload() Payload {
	return PayloadFromTransactions()
}

// PayloadFromTransactions creates a payload given a list of transaction hashes.
func PayloadFromTransactions(transactions ...*flow.TransactionBody) Payload {
	// avoid a nil transaction list
	if len(transactions) == 0 {
		transactions = []*flow.TransactionBody{}
	}
	return Payload{
		Collection: flow.Collection{
			Transactions: transactions,
		},
	}
}

// Hash returns the hash of the payload, simply the ID of the collection.
func (p Payload) Hash() flow.Identifier {
	return p.Collection.ID()
}

// PendingBlock is a wrapper type representing a block that cannot yet be
// processed. The block header, payload, and sender ID are stored together
// while waiting for the block to become processable.
type PendingBlock struct {
	OriginID flow.Identifier
	Header   *flow.Header
	Payload  *Payload
}

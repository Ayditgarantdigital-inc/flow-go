package stub

import (
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
)

// PendingMessage is a pending message to be sent
type PendingMessage struct {
	// The sender node id
	From      flow.Identifier
	ChannelID uint8
	Event     interface{}
	// The id of the receiver nodes
	TargetIDs []flow.Identifier
}

// Buffer buffers all the pending messages to be sent over the mock network from one node to a list of nodes
type Buffer struct {
	sync.Mutex
	pending []*PendingMessage
}

// NewBuffer initialize the Buffer
func NewBuffer() *Buffer {
	return &Buffer{
		pending: make([]*PendingMessage, 0),
	}
}

// Save stores a pending message to the buffer
func (b *Buffer) Save(from flow.Identifier, channelID uint8, event interface{}, targetIDs []flow.Identifier) {
	b.Lock()
	defer b.Unlock()
	b.pending = append(b.pending, &PendingMessage{
		From:      from,
		ChannelID: channelID,
		Event:     event,
		TargetIDs: targetIDs,
	})
}

// Flush recursively delivers the pending messages until the buffer is empty
func (b *Buffer) Flush(sendOne func(*PendingMessage) error) {
	for {
		toSend := b.takeAll()

		// This check is necessary to exit the endless forloop
		if len(toSend) == 0 {
			return
		}

		for _, msg := range toSend {
			_ = sendOne(msg)
		}
	}
}

// popAll takes all pending messages from the buffer and empty the buffer.
func (b *Buffer) takeAll() []*PendingMessage {
	b.Lock()
	defer b.Unlock()

	toSend := b.pending[:]
	b.pending = nil

	return toSend
}

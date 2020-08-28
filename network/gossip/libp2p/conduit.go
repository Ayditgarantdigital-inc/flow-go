package libp2p

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// SubmitFunc is a function that submits the given event for the given engine to
// the overlay network, which should take care of delivering it to the given
// recipients.
type SubmitFunc func(channelID uint8, event interface{}, targetIDs ...flow.Identifier) error

// PublishFunc is a function that broadcasts the specified event
// to all participants on the given channel.
type PublishFunc func(channelID uint8, event interface{}, selector flow.IdentityFilter) error

// UnicastFunc is a function that reliably sends the event via reliable 1-1 direct
// connections in  the underlying network to each of the target IDs.
type UnicastFunc func(channelID uint8, event interface{}, targetIDs ...flow.Identifier) error

// MulticastFunc is a function that reliably sends the event via reliable 1-1 direct
// connections in the underlying network to randomly chosen subset of nodes specified by the
// selector.
type MulticastFunc func(channelID uint8, event interface{}, num uint, selector flow.IdentityFilter) error

// Conduit is a helper of the overlay layer which functions as an accessor for
// sending messages within a single engine process. It sends all messages to
// what can be considered a bus reserved for that specific engine.
type Conduit struct {
	channelID uint8
	submit    SubmitFunc
	publish   PublishFunc
	unicast   UnicastFunc
	multicast MulticastFunc
}

// Submit will submit an event for delivery on the engine bus that is reserved
// for events of the engine it was initialized with.
func (c *Conduit) Submit(event interface{}, targetIDs ...flow.Identifier) error {
	return c.submit(c.channelID, event, targetIDs...)
}

// Publish sends an event to the network layer for unreliable delivery
// to subscribers of the given event on the network layer. It uses a
// publish-subscribe layer and can thus not guarantee that the specified
// recipients received the event.
func (c *Conduit) Publish(event interface{}, selector flow.IdentityFilter) error {
	return c.publish(c.channelID, event, selector)
}

// Unicast sends an event in a reliable way to the given recipients.
// It uses 1-1 direct messaging over the underlying network to deliver the event.
// It returns an error if unicasting to any of the target IDs fails.
func (c *Conduit) Unicast(event interface{}, targetIDs ...flow.Identifier) error {
	return c.unicast(c.channelID, event, targetIDs...)
}

// Multicast reliably sends the specified event to
// the specified number of recipients selected from the specified subset.
// The recipients are selected randomly from the set of identities selected by the selectors.
// In this context, reliable means that the event is sent across the network over a 1-1 direct messaging.
// It returns an error if it cannot send the event to a randomly chosen subset of nodes based on selector.
func (c *Conduit) Multicast(event interface{}, num uint, selector flow.IdentityFilter) error {
	return c.multicast(c.channelID, event, num, selector)
}

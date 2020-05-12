package mempool

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/verification/tracker"
)

// CollectionTrackers represents a concurrency-safe memory pool of collection trackers
type CollectionTrackers interface {

	// Has checks if the given collection ID has a tracker in mempool.
	Has(collID flow.Identifier) bool

	// Add will add the given collection tracker to the memory pool. It will
	// return false if it was already in the mempool.
	Add(collt *tracker.CollectionTracker) bool

	// Rem removes tracker with the given collection ID.
	Rem(collID flow.Identifier) bool

	// Inc atomically increases the counter of tracker by one and returns the updated tracker
	Inc(collID flow.Identifier) (*tracker.CollectionTracker, error)

	// ByCollectionID returns the collection tracker for the given collection ID
	ByCollectionID(collID flow.Identifier) (*tracker.CollectionTracker, bool)

	// All will return a list of collection trackers in mempool.
	All() []*tracker.CollectionTracker
}

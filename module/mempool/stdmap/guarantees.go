// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Guarantees implements the collections memory pool of the consensus nodes,
// used to store collection guarantees and to generate block payloads.
type Guarantees struct {
	*backend
}

// NewGuarantees creates a new memory pool for collection guarantees.
func NewGuarantees() (*Guarantees, error) {
	g := &Guarantees{
		backend: newBackend(),
	}

	return g, nil
}

// Add adds a collection guarantee guarantee to the mempool.
func (g *Guarantees) Add(guarantee *flow.CollectionGuarantee) error {
	return g.backend.Add(guarantee)
}

// Get returns the collection guarantee with the given ID from the mempool.
func (g *Guarantees) Get(collID flow.Identifier) (*flow.CollectionGuarantee, error) {
	entity, err := g.backend.Get(collID)
	if err != nil {
		return nil, err
	}
	guarantee, ok := entity.(*flow.CollectionGuarantee)
	if !ok {
		panic(fmt.Sprintf("invalid entity in guarantee pool (%T)", entity))
	}
	return guarantee, nil
}

// All returns all collection guarantees from the mempool.
func (g *Guarantees) All() []*flow.CollectionGuarantee {
	entities := g.backend.All()
	guarantees := make([]*flow.CollectionGuarantee, 0, len(entities))
	for _, entity := range entities {
		guarantee, ok := entity.(*flow.CollectionGuarantee)
		if !ok {
			panic(fmt.Sprintf("invalid entity in guarantee pool (%T)", entity))
		}
		guarantees = append(guarantees, guarantee)
	}
	return guarantees
}

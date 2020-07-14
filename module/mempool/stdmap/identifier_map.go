package stdmap

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/model"
)

// IdentifierMap represents a concurrency-safe memory pool for IdMapEntity.
type IdentifierMap struct {
	*Backend
}

// NewIdentifierMap creates a new memory pool for IdMapEntity.
func NewIdentifierMap(limit uint) (*IdentifierMap, error) {
	i := &IdentifierMap{
		Backend: NewBackend(WithLimit(limit)),
	}
	return i, nil
}

// Append will append the id to the list of identifiers associated with key.
// If the returned error is nil, the boolean value indicates whether the append was
// successful, or dropped since the id is already associated with the key.
func (i *IdentifierMap) Append(key, id flow.Identifier) (bool, error) {
	appended := false
	err := i.Backend.Run(func(backdata map[flow.Identifier]flow.Entity) error {
		var ids map[flow.Identifier]struct{}
		entity, ok := backdata[key]
		if !ok {
			// no record with key is available in the mempool,
			// initializes ids.
			ids = make(map[flow.Identifier]struct{})
		} else {
			idMapEntity, ok := entity.(model.IdMapEntity)
			if !ok {
				return fmt.Errorf("could not assert entity to IdMapEntity")
			}

			ids = idMapEntity.IDs
			if _, ok := ids[id]; ok {
				// id is already associated with the key
				// no need to append
				return nil
			}

			// removes map entry associated with key for update
			delete(backdata, key)
		}

		// appends id to the ids list
		ids[id] = struct{}{}

		// adds the new ids list associated with key to mempool
		idMapEntity := model.IdMapEntity{
			Key: key,
			IDs: ids,
		}

		backdata[key] = idMapEntity
		appended = true
		return nil
	})

	return appended, err
}

// Get returns list of all identifiers associated with key and true, if the key exists in the mempool.
// Otherwise it returns nil and false.
func (i *IdentifierMap) Get(key flow.Identifier) ([]flow.Identifier, bool) {
	entity, ok := i.Backend.ByID(key)
	if !ok {
		return nil, false
	}

	mapEntity, ok := entity.(model.IdMapEntity)
	if !ok {
		return nil, false
	}

	ids := make([]flow.Identifier, 0, len(mapEntity.IDs))
	for id := range mapEntity.IDs {
		ids = append(ids, id)
	}

	return ids, true
}

// Rem removes the given key with all associated identifiers.
func (i *IdentifierMap) Rem(id flow.Identifier) bool {
	return i.Backend.Rem(id)
}

// Size returns number of IdMapEntities in mempool
func (i *IdentifierMap) Size() uint {
	return i.Backend.Size()
}

// Keys returns a list of all keys in the mempool
func (i *IdentifierMap) Keys() ([]flow.Identifier, bool) {
	entities := i.Backend.All()
	keys := make([]flow.Identifier, 0)
	for _, entity := range entities {
		idMapEntity, ok := entity.(model.IdMapEntity)
		if !ok {
			return nil, false
		}
		keys = append(keys, idMapEntity.Key)
	}
	return keys, true
}

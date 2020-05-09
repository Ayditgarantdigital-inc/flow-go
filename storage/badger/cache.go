package badger

import (
	"fmt"
	"sync"

	"github.com/dapperlabs/flow-go/model/flow"
)

func withLimit(limit uint) func(*Cache) {
	return func(c *Cache) {
		c.limit = limit
	}
}

type storeFunc func(flow.Identifier, interface{}) error

func withStore(store storeFunc) func(*Cache) {
	return func(c *Cache) {
		c.store = store
	}
}

func noStore(flow.Identifier, interface{}) error {
	return fmt.Errorf("no store function for cache put available")
}

type retrieveFunc func(flow.Identifier) (interface{}, error)

func withRetrieve(retrieve retrieveFunc) func(*Cache) {
	return func(c *Cache) {
		c.retrieve = retrieve
	}
}

func noRetrieve(flow.Identifier) (interface{}, error) {
	return nil, fmt.Errorf("no retrieve function for cache get available")
}

type Cache struct {
	sync.RWMutex
	store    storeFunc
	retrieve retrieveFunc
	entities map[flow.Identifier]interface{}
	limit    uint
}

func newCache(options ...func(*Cache)) *Cache {
	c := Cache{
		store:    noStore,
		retrieve: noRetrieve,
		entities: make(map[flow.Identifier]interface{}),
		limit:    1000,
	}
	for _, option := range options {
		option(&c)
	}
	return &c
}

// Get will try to retrieve the entity from cache first, and then from the
// injected
func (c *Cache) Get(entityID flow.Identifier) (interface{}, error) {

	// check if we have it in the cache
	c.RLock()
	entity, cached := c.entities[entityID]
	c.RUnlock()
	if cached {
		return entity, nil
	}

	// get it from the database
	entity, err := c.retrieve(entityID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve entity: %w", err)
	}

	// cache the entity and eject a random one if we reached limit
	c.Lock()
	c.entities[entityID] = entity
	c.eject()
	c.Unlock()

	return entity, nil
}

// Put will add an entity to the cache with the given ID.
func (c *Cache) Put(entityID flow.Identifier, entity interface{}) error {

	// try to store the entity
	err := c.store(entityID, entity)
	if err != nil {
		return fmt.Errorf("could not store entity: %w", err)
	}

	// cache the entity and eject a random one if we reached limit
	c.Lock()
	c.entities[entityID] = entity
	c.eject()
	c.Unlock()

	return nil
}

// eject will check if we reached the limit and eject an entity if we did.
func (c *Cache) eject() {
	if uint(len(c.entities)) > c.limit {
		for ejectedID := range c.entities {
			delete(c.entities, ejectedID)
			break
		}
	}
}

// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package cache

import (
	"encoding/hex"
	"sync"

	"github.com/dapperlabs/flow-go/model/trickle"
)

// Cache implements a naive cache for peers.
type Cache struct {
	sync.Mutex
	caches map[uint8](map[string]*trickle.Response)
}

// New creates a new naive cache.
func New() (*Cache, error) {
	c := &Cache{
		caches: make(map[uint8](map[string]*trickle.Response)),
	}
	return c, nil
}

// Add will add a new engine cache.
func (c *Cache) Add(channelID uint8) {
	c.Lock()
	defer c.Unlock()
	c.caches[channelID] = make(map[string]*trickle.Response)
}

// Has returns whether we know the given ID.
func (c *Cache) Has(channelID uint8, eventID []byte) bool {
	c.Lock()
	defer c.Unlock()
	cache := c.caches[channelID]
	key := hex.EncodeToString(eventID)
	_, ok := cache[key]
	return ok
}

// Set sets the response for the given ID.
func (c *Cache) Set(channelID uint8, eventID []byte, res *trickle.Response) {
	c.Lock()
	defer c.Unlock()
	cache := c.caches[channelID]
	key := hex.EncodeToString(eventID)
	cache[key] = res
}

// Get returns the payload for the given ID.
func (c *Cache) Get(channelID uint8, eventID []byte) (*trickle.Response, bool) {
	c.Lock()
	defer c.Unlock()
	cache := c.caches[channelID]
	key := hex.EncodeToString(eventID)
	res, ok := cache[key]
	return res, ok
}

package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestReceive tests the receivedCache of the memoryCache
// The first two test cases are inserted into the cache and the rest two are not.
// The receive function is then evaluated against the test cases
func TestReceive(t *testing.T) {
	assert := assert.New(t)
	mhc := NewMemHashCache()

	//The sole keys that should exist in the cache
	initKeys := []string{
		"exists",
		"found",
	}

	for _, el := range initKeys {
		mhc.Receive(el)
	}

	tt := []struct {
		item  string
		found bool
	}{
		{ //an existing key in the cache
			item:  "exists",
			found: true,
		},
		{ //an existing key in the cache
			item:  "found",
			found: true,
		},
		{ //a non-existing key in the cache
			item:  "doesntexist",
			found: false,
		},
		{ //a non-existing key in the cache
			item:  "notfound",
			found: false,
		},
	}

	for _, tc := range tt {
		found := mhc.IsReceived(tc.item)
		assert.Equal(found, tc.found)
	}

}

// TestConfirm tests the confirm of the MemHashCache
// The first two test cases are inserted into the cache and the rest two are not.
// The confirm function is then evaluated against the test cases
func TestConfirm(t *testing.T) {
	assert := assert.New(t)
	mhc := NewMemHashCache()

	initKeys := []string{
		"exists",
		"found",
	}
	for _, el := range initKeys {
		mhc.Confirm(el)
	}

	tt := []struct {
		item  string
		found bool
	}{
		{ //an existing key in the cache
			item:  "exists",
			found: true,
		},
		{ //an existing key in the cache
			item:  "found",
			found: true,
		},
		{ //a non-existing key in the cache
			item:  "doesntexist",
			found: false,
		},
		{ //a non-existing key in the cache
			item:  "notfound",
			found: false,
		},
	}

	for _, tc := range tt {
		found := mhc.IsConfirmed(tc.item)
		assert.Equal(found, tc.found)
	}
}

// TestMemorySet tests the confirmCache of the memoryCache
// The first two test cases are inserted into the memory set and the rest two are not.
// The memorySet is then evaluated against the test cases
func TestMemorySet(t *testing.T) {
	ms := newMemorySet()

	initKeys := []string{
		"exists",
		"found",
	}

	for _, el := range initKeys {
		ms.put(el)
	}

	tt := []struct {
		item  string
		found bool
	}{
		{ //an existing key in the cache
			item:  "exists",
			found: true,
		},
		{ //an existing key in the cache
			item:  "found",
			found: true,
		},
		{ //a non-existing key in the cache
			item:  "doesntexist",
			found: false,
		},
		{ //a non-existing key in the cache
			item:  "notfound",
			found: false,
		},
	}

	for _, tc := range tt {
		found := ms.contains(tc.item)
		assert.Equal(t, found, tc.found)
	}
}

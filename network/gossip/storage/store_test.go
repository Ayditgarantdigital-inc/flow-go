package storage

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/proto/gossip/messages"
)

// TestDatabase covers several storage and retrieval of data to and from storage considering different scenarios
func TestDatabase(t *testing.T) {
	assert := assert.New(t)
	mmd := NewMemMsgStore()


	initKeys := []string{
		"exists",
		"found",
	}
	for _, key := range initKeys {
		err := mmd.Put(key, &messages.GossipMessage{})
		assert.Nil(err)
	}

	tt := []struct {
		item string
		err  error
	}{
		{ //an existing item
			item: "exists",
			err:  nil,
		},
		{ //an existing item
			item: "found",
			err:  nil,
		},
		{ //a non-existing item
			item: "doesntexist",
			err:  fmt.Errorf("non nil"),
		},
		{ //a non-existing item
			item: "notfound",
			err:  fmt.Errorf("non nil"),
		},
	}

	for _, tc := range tt {
		message, err := mmd.Get(tc.item)
		if err == tc.err {
			continue
		}
		if tc.err == nil {
			assert.Nil(err)
		}
		if tc.err != nil {
			assert.NotNil(err)
		}
		if message != nil {
			assert.Equal(string(message.MessageType), tc.item)
		}
		if message != nil && string(message.Payload) != tc.item {
			assert.Equal(string(message.MessageType), tc.item)
		}
	}
}

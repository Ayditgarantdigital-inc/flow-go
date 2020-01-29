package stub

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Hub is a value that stores mocked networks in order for them to send events directly
type Hub struct {
	isSync   bool
	networks map[flow.Identifier]*Network
	Buffer   *Buffer
}

// NewNetworkHub returns a MockHub value with empty network slice
func NewNetworkHub() *Hub {
	return &Hub{
		isSync:   false,
		networks: make(map[flow.Identifier]*Network),
		Buffer:   NewBuffer(),
	}
}

func (hub *Hub) EnableSyncDelivery() {
	hub.isSync = true
}

// GetNetwork returns the Network by the network ID (or node ID)
func (hub *Hub) GetNetwork(nodeID flow.Identifier) (*Network, bool) {
	net, ok := hub.networks[nodeID]
	return net, ok
}

// Plug stores the reference of the network in the hub object, in order for networks to find
// other network to send events directly
func (hub *Hub) Plug(net *Network) {
	hub.networks[net.GetID()] = net
}

// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package protocol

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Mutator represents an interface to modify the persistent protocol state in a
// way that conserves its integrity. It enforces a number of invariants on the
// input data that ensures the internal bookkeeping mechanisms remain functional
// and valid.
type Mutator interface {

	// Bootstrap initializes the persistent protocol state with the given
	// genesis block. A genesis block requires a number of zero, a hash of 32
	// zero bytes and an empty collection guarantees slice. The provided new
	// identities will be the initial staked nodes on the network.
	Bootstrap(genesis *flow.Block) error

	// Extend introduces the block with the given ID into the persistent
	// protocol state without modifying the current finalized state. It allows
	// us to execute fork-aware queries against ambiguous protocol state, while
	// still checking that the given block is a valid extension of the protocol
	// state.
	Extend(blockID flow.Identifier) error
}

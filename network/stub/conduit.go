package stub

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/trickle"
)

type Conduit struct {
	channelID uint8
	submit    trickle.SubmitFunc
}

func (c *Conduit) Submit(event interface{}, targetIDs ...flow.Identifier) error {
	return c.submit(c.channelID, event, targetIDs...)
}

package voterEvents

import "github.com/dapperlabs/flow-go/engine/consensus/eventdriven/modules/defConAct"

type Processor interface {
	OnSentVote(*defConAct.Vote)
}

type SentVoteConsumer interface {
	OnSentVote(*defConAct.Vote)
}

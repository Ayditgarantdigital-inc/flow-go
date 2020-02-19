package module

import (
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
)

// HotStuff defines the interface to the core HotStuff algorithm. It includes
// a method to start the event loop, and utilities to submit block proposals
// and votes received from other replicas.
type HotStuff interface {

	// Start starts the HotStuff event loop in a goroutine. It returns a
	// function to exit the loop and a channel that is closed when the event
	// loop exits for any reason.
	//
	// The exit function gracefully exits the loop. After it is called, no
	// further events will be accepted into the event queue. Any events
	// pending in the event queue will be drained and handled. Once the event
	// queue is empty, the event loop will exit.
	//
	// The done channel is closed when the event loop exits, either by calling
	// the exit function, or as a result of a fatal error.
	Start() (exit func(), done <-chan struct{})

	// SubmitProposal submits a new block proposal to the HotStuff event loop.
	// This method blocks until the proposal is accepted to the event queue.
	//
	// Block proposals must be submitted in order and only if they extend a
	// block already known to HotStuff core.
	SubmitProposal(proposal *flow.Header, parentView uint64)

	// SubmitVote submits a new vote to the HotStuff event loop.
	// This method blocks until the vote is accepted to the event queue.
	//
	// Votes may be submitted in any order.
	SubmitVote(originID flow.Identifier, blockID flow.Identifier, view uint64, sig crypto.Signature)
}

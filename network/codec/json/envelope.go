// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"
)

const (
	CodePing = iota
	CodePong
	CodeAuth
	CodeAnnounce
	CodeRequest
	CodeResponse
	CodeEcho

	// Consensus
	CodeBlockProposal
	CodeBlockVote
	CodeBlockCommit // ColdStuff-only

	CodeCollectionGuarantee
	CodeTransaction

	CodeBlock

	CodeCollectionRequest
	CodeCollectionResponse

	CodeExecutionRecipt

	CodeExecutionStateRequest
	CodeExecutionStateResponse
)

// Envelope is a wrapper to convey type information with JSON encoding without
// writing custom bytes to the wire.
type Envelope struct {
	Code uint8
	Data json.RawMessage
}

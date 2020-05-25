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

	// consensus
	CodeBlockProposal
	CodeBlockVote
	CodeBlockCommit // coldstuff-only

	// protocol state sync
	CodeSyncRequest
	CodeSyncResponse
	CodeRangeRequest
	CodeBatchRequest
	CodeBlockResponse

	// cluster consensus
	CodeClusterBlockProposal
	CodeClusterBlockVote
	CodeClusterBlockResponse

	CodeCollectionGuarantee
	CodeTransaction
	CodeTransactionBody

	CodeBlock

	CodeCollectionRequest
	CodeCollectionResponse

	CodeTransactionRequest
	CodeTransactionResponse

	CodeExecutionReceipt
	CodeExecutionStateRequest
	CodeExecutionStateResponse
	CodeExecutionStateSyncRequest
	CodeExecutionStateDelta
	CodeChunkDataPackRequest
	CodeChunkDataPackResponse
	CodeResultApproval
)

// Envelope is a wrapper to convey type information with JSON encoding without
// writing custom bytes to the wire.
type Envelope struct {
	Code uint8
	Data json.RawMessage
}

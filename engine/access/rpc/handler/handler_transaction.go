package handler

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow/protobuf/go/flow/access"
	"github.com/dapperlabs/flow/protobuf/go/flow/entities"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage"
)

// SendTransaction forwards the transaction to the collection node
func (h *Handler) SendTransaction(ctx context.Context, req *access.SendTransactionRequest) (*access.SendTransactionResponse, error) {

	// send the transaction to the collection node
	resp, err := h.collectionRPC.SendTransaction(ctx, req)
	if err != nil {
		return resp, status.Error(codes.Internal, fmt.Sprintf("failed to send transaction to a collection node: %v", err))
	}

	// convert the request message to a transaction
	tx, err := convert.MessageToTransaction(req.Transaction)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to convert transaction: %v", err))
	}

	// store the transaction locally
	err = h.transactions.Store(&tx)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to store transaction: %v", err))
	}

	return resp, nil
}

func (h *Handler) GetTransaction(_ context.Context, req *access.GetTransactionRequest) (*access.TransactionResponse, error) {

	id := flow.HashToID(req.Id)
	// look up transaction from storage
	tx, err := h.transactions.ByID(id)
	if err != nil {
		return nil, convertStorageError(err)
	}

	// derive status of the transaction
	status, err := h.deriveTransactionStatus(tx)
	if err != nil {
		return nil, convertStorageError(err)
	}

	// convert flow transaction to a protobuf message
	transaction := convert.TransactionToMessage(*tx)

	// set the status
	transaction.Status = status

	// return result
	resp := &access.TransactionResponse{
		Transaction: transaction,
	}
	return resp, nil
}

func (h *Handler) GetTransactionResult(ctx context.Context, req *access.GetTransactionRequest) (*access.TransactionResultResponse, error) {

	blockID := flow.HashToID(req.Id)
	// look up transaction from storage
	tx, err := h.transactions.ByID(blockID)
	if err != nil {
		return nil, convertStorageError(err)
	}

	txID := tx.ID()

	// get events for the transaction
	events, err := h.getTransactionEventsFromExecutionNode(ctx, blockID[:], txID[:])
	if err != nil {
		return nil, err
	}

	// derive status of the transaction
	status, err := h.deriveTransactionStatus(tx)
	if err != nil {
		return nil, convertStorageError(err)
	}

	// return result
	resp := &access.TransactionResultResponse{
		Status: status,
		Events: events,
	}

	return resp, nil
}

// deriveTransactionStatus derives the transaction status based on current protocol state
func (h *Handler) deriveTransactionStatus(tx *flow.TransactionBody) (entities.TransactionStatus, error) {

	collection, err := h.collections.LightByTransactionID(tx.ID())

	if errors.Is(err, storage.ErrNotFound) {
		// tx found in transaction storage but not in the collection storage
		return entities.TransactionStatus_STATUS_PENDING, nil
	}
	if err != nil {
		return entities.TransactionStatus_STATUS_UNKNOWN, err
	}

	block, err := h.blocks.ByCollectionID(collection.ID())

	if errors.Is(err, storage.ErrNotFound) {
		// tx found in transaction storage and collection storage but not in block storage
		// However, this will not happen as of now since the ingestion engine doesn't subscribe
		// for collections
		return entities.TransactionStatus_STATUS_PENDING, nil
	}
	if err != nil {
		return entities.TransactionStatus_STATUS_UNKNOWN, err
	}

	// get the latest sealed block from the state
	latestSealedBlock, err := h.getLatestSealedHeader()
	if err != nil {
		return entities.TransactionStatus_STATUS_UNKNOWN, err
	}

	// if the finalized block precedes the latest sealed block, then it can be safely assumed that it would have been
	// sealed as well and the transaction can be considered sealed
	if block.Height <= latestSealedBlock.Height {
		return entities.TransactionStatus_STATUS_SEALED, nil
	}
	// otherwise, the finalized block of the transaction has not yet been sealed
	// hence the transaction is finalized but not sealed
	return entities.TransactionStatus_STATUS_FINALIZED, nil
}

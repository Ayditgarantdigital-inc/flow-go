package ingress

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/network/mock"
	access "github.com/dapperlabs/flow-go/protobuf/services/access"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestSubmitTransaction(t *testing.T) {
	engine := mock.Engine{}

	h := handler{
		engine: &engine,
	}

	tx := unittest.TransactionBodyFixture(func(tb *flow.TransactionBody) {
		tb.Nonce = 0
		tb.ComputeLimit = 0
	})

	t.Run("should submit transaction to engine", func(t *testing.T) {
		engine.On("ProcessLocal", &tx).Return(nil).Once()

		res, err := h.SendTransaction(context.Background(), &access.SendTransactionRequest{
			Transaction: convert.TransactionToMessage(tx),
		})
		require.NoError(t, err)

		// should submit the transaction to the engine
		engine.AssertCalled(t, "ProcessLocal", &tx)

		// should return the fingerprint of the submitted transaction
		assert.Equal(t, tx.ID(), flow.HashToID(res.Id))
	})

	t.Run("should pass through error", func(t *testing.T) {
		expected := errors.New("error")
		engine.On("ProcessLocal", &tx).Return(expected).Once()

		res, err := h.SendTransaction(context.Background(), &access.SendTransactionRequest{
			Transaction: convert.TransactionToMessage(tx),
		})
		if assert.Error(t, err) {
			assert.Equal(t, expected, err)
		}

		// should submit the transaction to the engine
		engine.AssertCalled(t, "ProcessLocal", &tx)

		// should only return the error
		assert.Nil(t, res)
	})
}

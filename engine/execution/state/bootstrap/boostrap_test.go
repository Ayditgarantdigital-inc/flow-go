package bootstrap

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/storage/ledger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestGenerateGenesisStateCommitment(t *testing.T) {
	unittest.RunWithTempDir(t, func(dbDir string) {

		ls, err := ledger.NewTrieStorage(dbDir)
		require.NoError(t, err)

		newStateCommitment, err := BootstrapLedger(ls)
		require.NoError(t, err)
		require.True(t, bytes.Equal(flow.GenesisStateCommitment, newStateCommitment))
	})
}

package run

import (
	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/engine/execution/state/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/storage/ledger"
)

// NOTE: this is now unused and should become part of another tool.
func GenerateServiceAccountPrivateKey(seed []byte) (flow.AccountPrivateKey, error) {
	priv, err := crypto.GeneratePrivateKey(crypto.ECDSASecp256k1, seed)
	if err != nil {
		return flow.AccountPrivateKey{}, err
	}

	return flow.AccountPrivateKey{
		PrivateKey: priv,
		SignAlgo:   crypto.ECDSASecp256k1,
		HashAlgo:   hash.SHA2_256,
	}, nil
}

// NOTE: this is now unused and should become part of another tool.
func GenerateExecutionState(dbDir string, accountKey flow.AccountPublicKey, tokenSupply uint64, chain flow.Chain) (flow.StateCommitment, error) {
	metricsCollector := &metrics.NoopCollector{}
	ledgerStorage, err := ledger.NewMTrieStorage(dbDir, 100, metricsCollector, nil)
	defer ledgerStorage.CloseStorage()
	if err != nil {
		return nil, err
	}
	return bootstrap.NewBootstrapper(zerolog.Nop()).BootstrapLedger(ledgerStorage, accountKey, tokenSupply, chain)
}

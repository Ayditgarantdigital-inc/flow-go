package extract

import (
	"encoding/hex"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/onflow/flow-go/cmd/util/cmd/common"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/storage/badger"
)

var (
	flagExecutionStateDir string
	flagOutputDir         string
	flagBlockHash         string
	flagStateCommitment   string
	flagDatadir           string
	flagChainID           string
)

var Cmd = &cobra.Command{
	Use:   "execution-state-extract",
	Short: "Reads WAL files and generates the checkpoint containing state commitment for given block hash",
	Run:   run,
}

func init() {
	Cmd.Flags().StringVar(&flagExecutionStateDir, "execution-state-dir", "",
		"Execution Node state dir (where WAL logs are written")
	_ = Cmd.MarkFlagRequired("execution-state-dir")

	Cmd.Flags().StringVar(&flagOutputDir, "output-dir", "",
		"Directory to write new Execution State to")
	_ = Cmd.MarkFlagRequired("output-dir")

	Cmd.Flags().StringVar(&flagStateCommitment, "state-commitment", "",
		"state commitment (hex-encoded, 64 characters)")
	// _ = Cmd.MarkFlagRequired("block-hash")

	Cmd.Flags().StringVar(&flagBlockHash, "block-hash", "",
		"Block hash (hex-encoded, 64 characters)")
	// _ = Cmd.MarkFlagRequired("block-hash")

	Cmd.Flags().StringVar(&flagDatadir, "datadir", "",
		"directory that stores the protocol state")
	// _ = Cmd.MarkFlagRequired("datadir")

	Cmd.Flags().StringVar(&flagChainID, "chainID", "main",
		"chainID (main or test")
	_ = Cmd.MarkFlagRequired("chainID")
}

func run(*cobra.Command, []string) {
	var stateCommitment []byte

	if len(flagBlockHash) > 0 {
		blockID, err := flow.HexStringToIdentifier(flagBlockHash)
		if err != nil {
			log.Fatal().Err(err).Msg("malformed block hash")
		}

		db := common.InitStorage(flagDatadir)
		defer db.Close()

		cache := &metrics.NoopCollector{}
		commits := badger.NewCommits(cache, db)

		stateCommitment, err = getStateCommitment(commits, blockID)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot get state commitment for block")
		}
	}

	if len(flagStateCommitment) > 0 {
		var err error
		stateCommitment, err = hex.DecodeString(flagStateCommitment)
		if err != nil {
			log.Fatal().Err(err).Msg("cannot get decode the state commitment")
		}
	}

	log.Info().Msgf("Block state commitment: %s", hex.EncodeToString(stateCommitment))

	err := extractExecutionState(flagExecutionStateDir, stateCommitment, flagOutputDir, log.Logger, parseChainID(flagChainID))
	if err != nil {
		log.Fatal().Err(err).Msgf("error extracting the execution state: %w", err)
	}
}

func parseChainID(chainID string) flow.ChainID {
	switch chainID {
	case "main":
		return flow.Mainnet
	case "test":
		return flow.Testnet
	default:
		log.Fatal().Str("chain_id", chainID).Msg("invalid chain ID")
		return ""
	}
}

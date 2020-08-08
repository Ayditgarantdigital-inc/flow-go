package main

import (
	"fmt"

	"github.com/spf13/pflag"
	"google.golang.org/grpc"

	"github.com/onflow/flow/protobuf/go/flow/access"
	"github.com/onflow/flow/protobuf/go/flow/execution"

	"github.com/dapperlabs/flow-go/cmd"
	"github.com/dapperlabs/flow-go/consensus"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee/leader"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	recovery "github.com/dapperlabs/flow-go/consensus/recovery/protocol"
	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/engine/access/ingestion"
	pingeng "github.com/dapperlabs/flow-go/engine/access/ping"
	"github.com/dapperlabs/flow-go/engine/access/rpc"
	followereng "github.com/dapperlabs/flow-go/engine/common/follower"
	"github.com/dapperlabs/flow-go/engine/common/requester"
	synceng "github.com/dapperlabs/flow-go/engine/common/synchronization"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/buffer"
	finalizer "github.com/dapperlabs/flow-go/module/finalizer/consensus"
	"github.com/dapperlabs/flow-go/module/mempool/stdmap"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/signature"
	"github.com/dapperlabs/flow-go/module/synchronization"
	storage "github.com/dapperlabs/flow-go/storage/badger"
	grpcutils "github.com/dapperlabs/flow-go/utils/grpc"
)

func main() {

	var (
		blockLimit                   uint
		collectionLimit              uint
		receiptLimit                 uint
		pingEnabled                  bool
		ingestEng                    *ingestion.Engine
		requestEng                   *requester.Engine
		followerEng                  *followereng.Engine
		syncCore                     *synchronization.Core
		rpcConf                      rpc.Config
		rpcEng                       *rpc.Engine
		collectionRPC                access.AccessAPIClient
		executionRPC                 execution.ExecutionAPIClient
		err                          error
		conCache                     *buffer.PendingBlocks // pending block cache for follower
		transactionTimings           *stdmap.TransactionTimings
		collectionsToMarkFinalized   *stdmap.Times
		collectionsToMarkExecuted    *stdmap.Times
		blocksToMarkExecuted         *stdmap.Times
		transactionMetrics           module.TransactionMetrics
		logTxTimeToFinalized         bool
		logTxTimeToExecuted          bool
		logTxTimeToFinalizedExecuted bool
	)

	cmd.FlowNode(flow.RoleAccess.String()).
		ExtraFlags(func(flags *pflag.FlagSet) {
			flags.UintVar(&receiptLimit, "receipt-limit", 1000, "maximum number of execution receipts in the memory pool")
			flags.UintVar(&collectionLimit, "collection-limit", 1000, "maximum number of collections in the memory pool")
			flags.UintVar(&blockLimit, "block-limit", 1000, "maximum number of result blocks in the memory pool")
			flags.StringVarP(&rpcConf.GRPCListenAddr, "rpc-addr", "r", "localhost:9000", "the address the gRPC server listens on")
			flags.StringVarP(&rpcConf.HTTPListenAddr, "http-addr", "h", "localhost:8000", "the address the http proxy server listens on")
			flags.StringVarP(&rpcConf.CollectionAddr, "ingress-addr", "i", "localhost:9000", "the address (of the collection node) to send transactions to")
			flags.StringVarP(&rpcConf.ExecutionAddr, "script-addr", "s", "localhost:9000", "the address (of the execution node) forward the script to")
			flags.BoolVar(&logTxTimeToFinalized, "log-tx-time-to-finalized", false, "log transaction time to finalized")
			flags.BoolVar(&logTxTimeToExecuted, "log-tx-time-to-executed", false, "log transaction time to executed")
			flags.BoolVar(&logTxTimeToFinalizedExecuted, "log-tx-time-to-finalized-executed", false, "log transaction time to finalized and executed")
			flags.BoolVar(&pingEnabled, "ping-enabled", false, "whether to enable the ping process that pings all other peers and report the connectivity to metrics")
		}).
		Module("collection node client", func(node *cmd.FlowNodeBuilder) error {
			node.Logger.Info().Err(err).Msgf("Collection node Addr: %s", rpcConf.CollectionAddr)

			collectionRPCConn, err := grpc.Dial(
				rpcConf.CollectionAddr,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithInsecure())
			if err != nil {
				return err
			}
			collectionRPC = access.NewAccessAPIClient(collectionRPCConn)
			return nil
		}).
		Module("execution node client", func(node *cmd.FlowNodeBuilder) error {
			node.Logger.Info().Err(err).Msgf("Execution node Addr: %s", rpcConf.ExecutionAddr)

			executionRPCConn, err := grpc.Dial(
				rpcConf.ExecutionAddr,
				grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(grpcutils.DefaultMaxMsgSize)),
				grpc.WithInsecure())
			if err != nil {
				return err
			}
			executionRPC = execution.NewExecutionAPIClient(executionRPCConn)
			return nil
		}).
		Module("block cache", func(node *cmd.FlowNodeBuilder) error {
			conCache = buffer.NewPendingBlocks()
			return nil
		}).
		Module("sync core", func(node *cmd.FlowNodeBuilder) error {
			syncCore, err = synchronization.New(node.Logger, synchronization.DefaultConfig())
			return err
		}).
		Module("transaction timing mempools", func(node *cmd.FlowNodeBuilder) error {
			transactionTimings, err = stdmap.NewTransactionTimings(1500 * 300) // assume 1500 TPS * 300 seconds
			if err != nil {
				return err
			}

			collectionsToMarkFinalized, err = stdmap.NewTimes(50 * 300) // assume 50 collection nodes * 300 seconds
			if err != nil {
				return err
			}

			collectionsToMarkExecuted, err = stdmap.NewTimes(50 * 300) // assume 50 collection nodes * 300 seconds
			if err != nil {
				return err
			}

			blocksToMarkExecuted, err = stdmap.NewTimes(1 * 300) // assume 1 block per second * 300 seconds
			return err
		}).
		Module("transaction metrics", func(node *cmd.FlowNodeBuilder) error {
			transactionMetrics = metrics.NewTransactionCollector(transactionTimings, node.Logger, logTxTimeToFinalized,
				logTxTimeToExecuted, logTxTimeToFinalizedExecuted)
			return nil
		}).
		Component("RPC engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			rpcEng = rpc.New(node.Logger, node.State, rpcConf, executionRPC, collectionRPC, node.Storage.Blocks, node.Storage.Headers, node.Storage.Collections, node.Storage.Transactions, node.RootChainID, transactionMetrics)
			return rpcEng, nil
		}).
		Component("ingestion engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			requestEng, err = requester.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				engine.RequestCollections,
				filter.HasRole(flow.RoleCollection),
				func() flow.Entity { return &flow.Collection{} },
			)
			if err != nil {
				return nil, fmt.Errorf("could not create requester engine: %w", err)
			}
			ingestEng, err = ingestion.New(node.Logger, node.Network, node.State, node.Me, requestEng, node.Storage.Blocks, node.Storage.Headers, node.Storage.Collections, node.Storage.Transactions, transactionMetrics,
				collectionsToMarkFinalized, collectionsToMarkExecuted, blocksToMarkExecuted, rpcEng)
			requestEng.WithHandle(ingestEng.OnCollection)
			return ingestEng, err
		}).
		Component("requester engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			// We initialize the requester engine inside the ingestion engine due to the mutual dependency. However, in
			// order for it to properly start and shut down, we should still return it as its own engine here, so it can
			// be handled by the scaffold.
			return requestEng, nil
		}).
		Component("follower engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {

			// initialize cleaner for DB
			cleaner := storage.NewCleaner(node.Logger, node.DB, metrics.NewCleanerCollector(), flow.DefaultValueLogGCFrequency)

			// create a finalizer that will handle updating the protocol
			// state when the follower detects newly finalized blocks
			final := finalizer.NewFinalizer(node.DB, node.Storage.Headers, node.State)

			// initialize the staking & beacon verifiers, signature joiner
			staking := signature.NewAggregationVerifier(encoding.ConsensusVoteTag)
			beacon := signature.NewThresholdVerifier(encoding.RandomBeaconTag)
			merger := signature.NewCombiner()

			// initialize and pre-generate leader selections from the seed
			selection, err := leader.NewSelectionForConsensus(leader.EstimatedSixMonthOfViews, node.RootBlock.Header, node.RootQC, node.State)
			if err != nil {
				return nil, fmt.Errorf("could not create leader selection for main consensus: %w", err)
			}

			// initialize consensus committee's membership state
			// This committee state is for the HotStuff follower, which follows the MAIN CONSENSUS Committee
			// Note: node.Me.NodeID() is not part of the consensus committee
			mainConsensusCommittee, err := committee.NewMainConsensusCommitteeState(node.State, node.Me.NodeID(), selection)
			if err != nil {
				return nil, fmt.Errorf("could not create Committee state for main consensus: %w", err)
			}

			// initialize the verifier for the protocol consensus
			verifier := verification.NewCombinedVerifier(mainConsensusCommittee, node.DKGState, staking, beacon, merger)

			finalized, pending, err := recovery.FindLatest(node.State, node.Storage.Headers)
			if err != nil {
				return nil, fmt.Errorf("could not find latest finalized block and pending blocks to recover consensus follower: %w", err)
			}

			// creates a consensus follower with ingestEngine as the notifier
			// so that it gets notified upon each new finalized block
			followerCore, err := consensus.NewFollower(node.Logger, mainConsensusCommittee, node.Storage.Headers, final, verifier, ingestEng, node.RootBlock.Header, node.RootQC, finalized, pending)
			if err != nil {
				return nil, fmt.Errorf("could not initialize follower core: %w", err)
			}

			followerEng, err = followereng.New(
				node.Logger,
				node.Network,
				node.Me,
				node.Metrics.Engine,
				node.Metrics.Mempool,
				cleaner,
				node.Storage.Headers,
				node.Storage.Payloads,
				node.State,
				conCache,
				followerCore,
				syncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create follower engine: %w", err)
			}

			return followerEng, nil
		}).
		Component("sync engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			sync, err := synceng.New(
				node.Logger,
				node.Metrics.Engine,
				node.Network,
				node.Me,
				node.State,
				node.Storage.Blocks,
				followerEng,
				syncCore,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create synchronization engine: %w", err)
			}
			return sync, nil
		}).
		Component("ping engine", func(node *cmd.FlowNodeBuilder) (module.ReadyDoneAware, error) {
			ping, err := pingeng.New(
				node.Logger,
				node.State,
				node.Me,
				pingEnabled,
				node.Middleware,
			)
			if err != nil {
				return nil, fmt.Errorf("could not create ping engine: %w", err)
			}
			return ping, nil
		}).
		Run()
}

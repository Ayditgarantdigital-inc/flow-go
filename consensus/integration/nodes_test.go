package integration_test

import (
	"os"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus"
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	synceng "github.com/onflow/flow-go/engine/common/synchronization"
	"github.com/onflow/flow-go/engine/consensus/compliance"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module/buffer"
	builder "github.com/onflow/flow-go/module/builder/consensus"
	finalizer "github.com/onflow/flow-go/module/finalizer/consensus"
	"github.com/onflow/flow-go/module/local"
	consensusMempools "github.com/onflow/flow-go/module/mempool/consensus"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/module/metrics"
	synccore "github.com/onflow/flow-go/module/synchronization"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/network/mocknetwork"
	"github.com/onflow/flow-go/state/protocol"
	bprotocol "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/state/protocol/util"
	storage "github.com/onflow/flow-go/storage/badger"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

const hotstuffTimeout = 100 * time.Millisecond

type Node struct {
	db         *badger.DB
	dbDir      string
	index      int
	log        zerolog.Logger
	id         *flow.Identity
	compliance *compliance.Engine
	sync       *synceng.Engine
	hot        *hotstuff.EventLoop
	state      *bprotocol.MutableState
	headers    *storage.Headers
	net        *Network
}

func (n *Node) Shutdown() {
	<-n.sync.Done()
	<-n.compliance.Done()
}

// n - the total number of nodes to be created
// finalizedCount - the number of finalized blocks before stopping the tests
// tolerate - the number of node to tolerate that don't need to reach the finalization count
// 						before stopping the tests
func createNodes(t *testing.T, stopper *Stopper, rootSnapshot protocol.Snapshot) ([]*Node, *Hub) {

	consensus, err := rootSnapshot.Identities(filter.HasRole(flow.RoleConsensus))
	require.NoError(t, err)

	hub := NewNetworkHub()
	nodes := make([]*Node, 0, len(consensus))
	for i, identity := range consensus {
		node := createNode(t, i, identity, rootSnapshot, hub, stopper)
		nodes = append(nodes, node)
	}

	return nodes, hub
}

func createRootSnapshot(t *testing.T, n int) *inmem.Snapshot {

	// create n consensus node participants
	consensus := unittest.IdentityListFixture(n, unittest.WithRole(flow.RoleConsensus))
	// add other roles to create a complete identity list
	participants := unittest.CompleteIdentitySet(consensus...)

	root, result, seal := unittest.BootstrapFixture(participants)
	rootQC := &flow.QuorumCertificate{
		View:      root.Header.View,
		BlockID:   root.ID(),
		SignerIDs: consensus.NodeIDs(), // all participants sign root block
		SigData:   unittest.CombinedSignatureFixture(2),
	}

	rootSnapshot, err := inmem.SnapshotFromBootstrapState(root, result, seal, rootQC)
	require.NoError(t, err)

	return rootSnapshot
}

func createNode(
	t *testing.T,
	index int,
	identity *flow.Identity,
	rootSnapshot protocol.Snapshot,
	hub *Hub,
	stopper *Stopper,
) *Node {

	db, dbDir := unittest.TempBadgerDB(t)
	metrics := metrics.NewNoopCollector()
	tracer := trace.NewNoopTracer()

	headersDB := storage.NewHeaders(metrics, db)
	guaranteesDB := storage.NewGuarantees(metrics, db)
	sealsDB := storage.NewSeals(metrics, db)
	indexDB := storage.NewIndex(metrics, db)
	resultsDB := storage.NewExecutionResults(metrics, db)
	receiptsDB := storage.NewExecutionReceipts(metrics, db, resultsDB)
	payloadsDB := storage.NewPayloads(db, indexDB, guaranteesDB, sealsDB, receiptsDB, resultsDB)
	blocksDB := storage.NewBlocks(db, headersDB, payloadsDB)
	setupsDB := storage.NewEpochSetups(metrics, db)
	commitsDB := storage.NewEpochCommits(metrics, db)
	statusesDB := storage.NewEpochStatuses(metrics, db)
	consumer := events.NewNoop()

	state, err := bprotocol.Bootstrap(metrics, db, headersDB, sealsDB, resultsDB, blocksDB, setupsDB, commitsDB, statusesDB, rootSnapshot)
	require.NoError(t, err)

	fullState, err := bprotocol.NewFullConsensusState(state, indexDB, payloadsDB, tracer, consumer, util.MockReceiptValidator(), util.MockSealValidator(sealsDB))
	require.NoError(t, err)

	localID := identity.ID()

	node := &Node{
		db:    db,
		dbDir: dbDir,
		index: index,
		id:    identity,
	}

	// log with node index an ID
	log := unittest.Logger().With().
		Int("index", index).
		Hex("node_id", localID[:]).
		Logger()

	stopConsumer := stopper.AddNode(node)

	counterConsumer := &CounterConsumer{
		finalized: func(total uint) {
			stopper.onFinalizedTotal(node.id.ID(), total)
		},
	}

	// log with node index
	notifier := notifications.NewLogConsumer(log)
	dis := pubsub.NewDistributor()
	dis.AddConsumer(stopConsumer)
	dis.AddConsumer(counterConsumer)
	dis.AddConsumer(notifier)

	cleaner := &storagemock.Cleaner{}
	cleaner.On("RunGC")

	// make local
	priv := helper.MakeBLSKey(t)
	me, err := local.New(identity, priv)
	require.NoError(t, err)

	// add a network for this node to the hub
	net := hub.AddNetwork(localID, node)

	guaranteeLimit, sealLimit := uint(1000), uint(1000)
	guarantees, err := stdmap.NewGuarantees(guaranteeLimit)
	require.NoError(t, err)

	receipts := consensusMempools.NewExecutionTree()

	seals := stdmap.NewIncorporatedResultSeals(stdmap.WithLimit(sealLimit))

	// initialize the block builder
	build := builder.NewBuilder(metrics, db, fullState, headersDB, sealsDB, indexDB, blocksDB, resultsDB,
		guarantees, seals, receipts, tracer)

	signer := &Signer{identity.ID()}

	// initialize the pending blocks cache
	cache := buffer.NewPendingBlocks()

	rootHeader, err := rootSnapshot.Head()
	require.NoError(t, err)

	rootQC, err := rootSnapshot.QuorumCertificate()
	require.NoError(t, err)

	// selector := filter.HasRole(flow.RoleConsensus)
	committee, err := committees.NewConsensusCommittee(state, localID)
	require.NoError(t, err)

	// initialize the block finalizer
	final := finalizer.NewFinalizer(db, headersDB, fullState)

	// initialize the persister
	persist := persister.New(db, rootHeader.ChainID)

	prov := &mocknetwork.Engine{}
	prov.On("SubmitLocal", mock.Anything).Return(nil)

	syncCore, err := synccore.New(log, synccore.DefaultConfig())
	require.NoError(t, err)

	// initialize the compliance engine
	compCore, err := compliance.NewCore(log, metrics, tracer, metrics, metrics, cleaner, headersDB, payloadsDB, fullState, cache, syncCore)
	require.NoError(t, err)

	comp, err := compliance.NewEngine(log, net, me, prov, compCore)
	require.NoError(t, err)

	// initialize the synchronization engine
	sync, err := synceng.New(log, metrics, net, me, state, blocksDB, comp, syncCore)
	require.NoError(t, err)

	pending := []*flow.Header{}
	// initialize the block finalizer
	hot, err := consensus.NewParticipant(log, dis, metrics, headersDB,
		committee, build, final, persist, signer, comp, rootHeader,
		rootQC, rootHeader, pending, consensus.WithInitialTimeout(hotstuffTimeout), consensus.WithMinTimeout(hotstuffTimeout))

	require.NoError(t, err)

	comp = comp.WithConsensus(hot)

	node.compliance = comp
	node.sync = sync
	node.state = fullState
	node.hot = hot
	node.headers = headersDB
	node.net = net
	node.log = log

	return node
}

func cleanupNodes(nodes []*Node) {
	for _, n := range nodes {
		_ = n.db.Close()
		_ = os.RemoveAll(n.dbDir)
	}
}

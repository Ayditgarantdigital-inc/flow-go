package unittest

import (
	crand "crypto/rand"
	"fmt"
	"math/rand"
	"time"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/execution"
	"github.com/dapperlabs/flow-go/engine/execution/state"
	"github.com/dapperlabs/flow-go/engine/verification"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/mempool/entity"
)

func AddressFixture() flow.Address {
	return flow.RootAddress
}

func AccountSignatureFixture() flow.AccountSignature {
	return flow.AccountSignature{
		Account:   AddressFixture(),
		Signature: []byte{1, 2, 3, 4},
	}
}

func BlockFixture() flow.Block {
	return BlockWithParentFixture(IdentifierFixture())
}

func BlockWithParentFixture(parentID flow.Identifier) flow.Block {
	payload := flow.Payload{
		Identities: IdentityListFixture(32),
		Guarantees: CollectionGuaranteesFixture(16),
	}
	header := BlockHeaderWithParentFixture(parentID)
	header.PayloadHash = payload.Hash()
	return flow.Block{
		Header:  header,
		Payload: payload,
	}
}

func BlockHeaderFixture() flow.Header {
	return BlockHeaderWithParentFixture(IdentifierFixture())
}

func BlockHeaderWithParentFixture(parentID flow.Identifier) flow.Header {
	return flow.Header{
		ChainID:        "chain",
		ParentID:       parentID,
		View:           rand.Uint64(),
		ParentVoterIDs: IdentifierListFixture(4),
		ParentVoterSig: SignatureFixture(),
		ProposerID:     IdentifierFixture(),
		ProposerSig:    SignatureFixture(),
	}
}

// BlockWithParent creates a new block that is valid
// with respect to the given parent block.
func BlockWithParent(parent *flow.Block) flow.Block {
	payload := flow.Payload{
		Identities: IdentityListFixture(32),
		Guarantees: CollectionGuaranteesFixture(16),
	}

	header := BlockHeaderFixture()
	header.View = parent.View + 1
	header.ChainID = parent.ChainID
	header.Timestamp = time.Now()
	header.ParentID = parent.ID()
	header.PayloadHash = payload.Hash()

	return flow.Block{
		Header:  header,
		Payload: payload,
	}
}

func SealFixture() flow.Seal {
	return flow.Seal{
		BlockID:       IdentifierFixture(),
		PreviousState: StateCommitmentFixture(),
		FinalState:    StateCommitmentFixture(),
		Signature:     SignatureFixture(),
	}
}

func ClusterBlockFixture() cluster.Block {
	payload := cluster.Payload{
		Collection: flow.LightCollection{
			Transactions: []flow.Identifier{IdentifierFixture()},
		},
	}
	header := BlockHeaderFixture()
	header.PayloadHash = payload.Hash()
	return cluster.Block{
		Header:  header,
		Payload: payload,
	}
}

// ClusterBlockWithParent creates a new cluster consensus block that is valid
// with respect to the given parent block.
func ClusterBlockWithParent(parent *cluster.Block) cluster.Block {
	payload := cluster.Payload{
		Collection: flow.LightCollection{
			Transactions: []flow.Identifier{IdentifierFixture()},
		},
	}

	header := BlockHeaderFixture()
	header.Height = parent.Height + 1
	header.View = parent.View + 1
	header.ChainID = parent.ChainID
	header.Timestamp = time.Now()
	header.ParentID = parent.ID()
	header.PayloadHash = payload.Hash()

	block := cluster.Block{
		Header:  header,
		Payload: payload,
	}

	return block
}

func CollectionGuaranteeFixture() *flow.CollectionGuarantee {
	return &flow.CollectionGuarantee{
		CollectionID: IdentifierFixture(),
		SignerIDs:    IdentifierListFixture(16),
		Signature:    SignatureFixture(),
	}
}

func CollectionGuaranteesFixture(n int) []*flow.CollectionGuarantee {
	ret := make([]*flow.CollectionGuarantee, 0, n)
	for i := 1; i <= n; i++ {
		cg := flow.CollectionGuarantee{
			CollectionID: IdentifierFixture(),
			Signature:    SignatureFixture(),
		}
		ret = append(ret, &cg)
	}
	return ret
}

func CollectionFixture(n int) flow.Collection {
	transactions := make([]*flow.TransactionBody, 0, n)

	for i := 0; i < n; i++ {
		tx := TransactionFixture()
		transactions = append(transactions, &tx.TransactionBody)
	}

	return flow.Collection{Transactions: transactions}
}

func ExecutionReceiptFixture() *flow.ExecutionReceipt {
	return &flow.ExecutionReceipt{
		ExecutorID:        IdentifierFixture(),
		ExecutionResult:   *ExecutionResultFixture(),
		Spocks:            nil,
		ExecutorSignature: SignatureFixture(),
	}
}

func StateViewFixture() *state.View {
	return state.NewView(func(key flow.RegisterID) (bytes []byte, err error) {
		return nil, nil
	})
}

func CompleteCollectionFixture() *entity.CompleteCollection {
	txBody := TransactionBodyFixture()
	return &entity.CompleteCollection{
		Guarantee: &flow.CollectionGuarantee{
			CollectionID: flow.Collection{Transactions: []*flow.TransactionBody{&txBody}}.ID(),
			Signature:    SignatureFixture(),
		},
		Transactions: []*flow.TransactionBody{&txBody},
	}
}

func ExecutableBlockFixture(collections int) *entity.ExecutableBlock {

	return ExecutableBlockFixtureWithParent(collections, IdentifierFixture())
}

func ExecutableBlockFixtureWithParent(collections int, parentID flow.Identifier) *entity.ExecutableBlock {

	completeCollections := make(map[flow.Identifier]*entity.CompleteCollection, collections)
	block := BlockWithParentFixture(parentID)
	block.Guarantees = nil

	for i := 0; i < collections; i++ {
		completeCollection := CompleteCollectionFixture()
		block.Guarantees = append(block.Guarantees, completeCollection.Guarantee)
		completeCollections[completeCollection.Guarantee.CollectionID] = completeCollection
	}

	block.PayloadHash = block.Payload.Hash()

	return &entity.ExecutableBlock{
		Block:               &block,
		CompleteCollections: completeCollections,
	}
}

func ComputationResultFixture(n int) *execution.ComputationResult {
	stateViews := make([]*state.View, n)
	for i := 0; i < n; i++ {
		stateViews[i] = StateViewFixture()
	}
	return &execution.ComputationResult{
		ExecutableBlock: ExecutableBlockFixture(n),
		StateViews:      stateViews,
	}
}

func ComputationResultForBlockFixture(executableBlock *entity.ExecutableBlock) *execution.ComputationResult {
	n := len(executableBlock.CompleteCollections)
	stateViews := make([]*state.View, n)
	for i := 0; i < n; i++ {
		stateViews[i] = StateViewFixture()
	}
	return &execution.ComputationResult{
		ExecutableBlock: executableBlock,
		StateViews:      stateViews,
	}
}

func ExecutionResultFixture() *flow.ExecutionResult {
	return &flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			PreviousResultID: IdentifierFixture(),
			BlockID:          IdentifierFixture(),
			FinalStateCommit: StateCommitmentFixture(),
			Chunks: flow.ChunkList{
				ChunkFixture(),
				ChunkFixture(),
			},
		},
		Signatures: SignaturesFixture(6),
	}
}

func WithExecutionResultID(id flow.Identifier) func(*flow.ResultApproval) {
	return func(ra *flow.ResultApproval) {
		ra.Body.ExecutionResultID = id
	}
}

func ResultApprovalFixture(opts ...func(*flow.ResultApproval)) *flow.ResultApproval {
	attestation := flow.Attestation{
		BlockID:           IdentifierFixture(),
		ExecutionResultID: IdentifierFixture(),
		ChunkIndex:        uint64(0),
	}

	approval := flow.ResultApproval{
		Body: flow.ResultApprovalBody{
			Attestation:          attestation,
			ApproverID:           IdentifierFixture(),
			AttestationSignature: SignatureFixture(),
			Spock:                nil,
		},
		VerifierSignature: SignatureFixture(),
	}

	for _, apply := range opts {
		apply(&approval)
	}

	return &approval
}

func StateCommitmentFixture() flow.StateCommitment {
	var state = make([]byte, 20)
	_, _ = rand.Read(state[0:20])
	return state
}

func HashFixture(size int) crypto.Hash {
	hash := make(crypto.Hash, size)
	for i := 0; i < size; i++ {
		hash[i] = byte(i)
	}
	return hash
}

func IdentifierListFixture(n int) []flow.Identifier {
	list := make([]flow.Identifier, n)
	for i := 0; i < n; i++ {
		list[i] = IdentifierFixture()
	}
	return list
}

func IdentifierFixture() flow.Identifier {
	var id flow.Identifier
	_, _ = rand.Read(id[:])
	return id
}

// WithRole adds a role to an identity fixture.
func WithRole(role flow.Role) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.Role = role
	}
}

func WithStake(stake uint64) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.Stake = stake
	}
}

func generateRandomSeed() []byte {
	seed := make([]byte, 48)
	if _, err := crand.Read(seed); err != nil {
		panic(err)
	}
	return seed
}

// IdentityFixture returns a node identity.
func IdentityFixture(opts ...func(*flow.Identity)) *flow.Identity {
	nodeID := IdentifierFixture()
	identity := flow.Identity{
		NodeID:  nodeID,
		Address: fmt.Sprintf("address-%v", nodeID[0:7]),
		Role:    flow.RoleConsensus,
		Stake:   1000,
	}
	for _, apply := range opts {
		apply(&identity)
	}
	return &identity
}

// WithNodeID adds a node ID with the given first byte to an identity.
func WithNodeID(b byte) func(*flow.Identity) {
	return func(identity *flow.Identity) {
		identity.NodeID = flow.Identifier{b}
	}
}

// WithRandomPublicKeys adds random public keys to an identity.
func WithRandomPublicKeys() func(*flow.Identity) {
	return func(identity *flow.Identity) {
		randBeac, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, generateRandomSeed())
		if err != nil {
			panic(err)
		}
		identity.RandomBeaconPubKey = randBeac.PublicKey()
		stak, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, generateRandomSeed())
		if err != nil {
			panic(err)
		}
		identity.StakingPubKey = stak.PublicKey()
		netw, err := crypto.GeneratePrivateKey(crypto.ECDSA_P256, generateRandomSeed())
		if err != nil {
			panic(err)
		}
		identity.NetworkPubKey = netw.PublicKey()
	}
}

// IdentityListFixture returns a list of node identity objects. The identities
// can be customized (ie. set their role) by passing in a function that modifies
// the input identities as required.
func IdentityListFixture(n int, opts ...func(*flow.Identity)) flow.IdentityList {
	identities := make(flow.IdentityList, n)

	for i := 0; i < n; i++ {
		identity := IdentityFixture()
		identity.Address = fmt.Sprintf("address-%d", i+1)
		for _, opt := range opts {
			opt(identity)
		}
		identities[i] = identity
	}

	return identities
}

func ChunkFixture() *flow.Chunk {
	return &flow.Chunk{
		ChunkBody: flow.ChunkBody{
			CollectionIndex:      42,
			StartState:           StateCommitmentFixture(),
			EventCollection:      IdentifierFixture(),
			TotalComputationUsed: 4200,
			NumberOfTransactions: 42,
		},
		Index:    0,
		EndState: StateCommitmentFixture(),
	}
}

func SignatureFixture() crypto.Signature {
	sig := make([]byte, 32)
	_, _ = rand.Read(sig)
	return sig
}

func SignaturesFixture(n int) []crypto.Signature {
	var sigs []crypto.Signature
	for i := 0; i < n; i++ {
		sigs = append(sigs, SignatureFixture())
	}
	return sigs
}

func TransactionFixture(n ...func(t *flow.Transaction)) flow.Transaction {
	tx := flow.Transaction{TransactionBody: TransactionBodyFixture()}
	if len(n) > 0 {
		n[0](&tx)
	}
	return tx
}

func TransactionBodyFixture(opts ...func(*flow.TransactionBody)) flow.TransactionBody {
	tb := flow.TransactionBody{
		Script:           []byte("pub fun main() {}"),
		ReferenceBlockID: IdentifierFixture(),
		Nonce:            rand.Uint64(),
		ComputeLimit:     10,
		PayerAccount:     AddressFixture(),
		ScriptAccounts:   []flow.Address{AddressFixture()},
		Signatures:       []flow.AccountSignature{AccountSignatureFixture()},
	}

	for _, apply := range opts {
		apply(&tb)
	}

	return tb
}

// VerifiableChunk returns a complete verifiable chunk with an
// execution receipt referencing the block/collections.
func VerifiableChunkFixture(chunkIndex uint64) *verification.VerifiableChunk {

	guarantees := make([]*flow.CollectionGuarantee, 0)

	var col flow.Collection

	for i := 0; i <= int(chunkIndex); i++ {
		col = CollectionFixture(1)
		guarantee := col.Guarantee()
		guarantees = append(guarantees, &guarantee)
	}

	payload := flow.Payload{
		Identities: IdentityListFixture(32),
		Guarantees: guarantees,
	}
	header := BlockHeaderFixture()
	header.PayloadHash = payload.Hash()

	block := flow.Block{
		Header:  header,
		Payload: payload,
	}

	chunks := make([]*flow.Chunk, 0)

	var chunk flow.Chunk

	for i := 0; i <= int(chunkIndex); i++ {
		chunk = flow.Chunk{
			ChunkBody: flow.ChunkBody{
				CollectionIndex: uint(i),
				StartState:      StateCommitmentFixture(),
			},
			Index: uint64(i),
		}
		chunks = append(chunks, &chunk)
	}

	result := flow.ExecutionResult{
		ExecutionResultBody: flow.ExecutionResultBody{
			BlockID: block.ID(),
			Chunks:  chunks,
		},
	}

	receipt := flow.ExecutionReceipt{
		ExecutionResult: result,
	}

	return &verification.VerifiableChunk{
		ChunkIndex: chunkIndex,
		EndState:   StateCommitmentFixture(),
		Block:      &block,
		Receipt:    &receipt,
		Collection: &col,
	}
}

func ChunkHeaderFixture() flow.ChunkHeader {
	return flow.ChunkHeader{
		ChunkID:     IdentifierFixture(),
		StartState:  StateCommitmentFixture(),
		RegisterIDs: []flow.RegisterID{{1}, {2}, {3}},
	}
}

func ChunkDataPackFixture(identifier flow.Identifier) flow.ChunkDataPack {
	return flow.ChunkDataPack{
		ChunkID:         identifier,
		StartState:      StateCommitmentFixture(),
		RegisterTouches: []flow.RegisterTouch{flow.RegisterTouch{RegisterID: []byte{'1'}, Value: []byte{'a'}, Proof: []byte{'p'}}},
	}
}

// SeedFixture returns a random []byte with length n
func SeedFixture(n int) []byte {
	var seed = make([]byte, n)
	_, _ = rand.Read(seed[0:n])
	return seed
}

// SeedFixtures returns a list of m random []byte, each having length n
func SeedFixtures(m int, n int) [][]byte {
	var seeds = make([][]byte, m, n)
	for i := range seeds {
		seeds[i] = SeedFixture(n)
	}
	return seeds
}

// EventFixture returns an event
func EventFixture(eType flow.EventType, transactionIndex uint32, eventIndex uint32, txID flow.Identifier) flow.Event {
	return flow.Event{
		Type:             eType,
		TransactionIndex: transactionIndex,
		EventIndex:       eventIndex,
		Payload:          []byte{},
		TransactionID:    txID,
	}
}

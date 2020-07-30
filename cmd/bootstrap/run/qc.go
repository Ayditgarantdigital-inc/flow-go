package run

import (
	"fmt"
	"io/ioutil"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/committee/leader"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/encoding"
	"github.com/dapperlabs/flow-go/model/epoch"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/metrics"
	"github.com/dapperlabs/flow-go/module/signature"
	"github.com/dapperlabs/flow-go/state/protocol"
	protoBadger "github.com/dapperlabs/flow-go/state/protocol/badger"
	storeBadger "github.com/dapperlabs/flow-go/storage/badger"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Participant struct {
	bootstrap.NodeInfo
	RandomBeaconPrivKey crypto.PrivateKey
}

type ParticipantData struct {
	Commit       *epoch.Commit
	Participants []Participant
}

func GenerateRootQC(participantData ParticipantData, block *flow.Block) (*model.QuorumCertificate, error) {
	state, db, err := NewProtocolState(block)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	validators, signers, err := createValidators(state, participantData, block)
	if err != nil {
		return nil, err
	}

	hotBlock := model.Block{
		BlockID:     block.ID(),
		View:        block.Header.View,
		ProposerID:  block.Header.ProposerID,
		QC:          nil,
		PayloadHash: block.Header.PayloadHash,
		Timestamp:   block.Header.Timestamp,
	}

	votes := make([]*model.Vote, 0, len(signers))
	for _, signer := range signers {
		vote, err := signer.CreateVote(&hotBlock)
		if err != nil {
			return nil, err
		}
		votes = append(votes, vote)
	}

	// manually aggregate sigs
	qc, err := signers[0].CreateQC(votes)
	if err != nil {
		return nil, err
	}

	// validate QC
	err = validators[0].ValidateQC(qc, &hotBlock)

	return qc, err
}

func createValidators(ps protocol.State, participantData ParticipantData, block *flow.Block) ([]hotstuff.Validator, []hotstuff.Signer, error) {
	n := len(participantData.Participants)

	groupSize := uint(len(participantData.Commit.DKGParticipants))
	if groupSize < uint(n) {
		return nil, nil, fmt.Errorf("need at least as many signers as DKG participants, got %v and %v", groupSize, n)
	}

	signers := make([]hotstuff.Signer, n)
	validators := make([]hotstuff.Validator, n)

	forks := &mocks.ForksReader{}

	for i, participant := range participantData.Participants {
		// get the participant private keys
		keys, err := participant.PrivateKeys()
		if err != nil {
			return nil, nil, fmt.Errorf("could not get private keys for participant: %w", err)
		}

		local, err := local.New(participant.Identity(), keys.StakingKey)
		if err != nil {
			return nil, nil, err
		}

		selection := leader.NewSelectionForBootstrap()

		// create consensus committee's state
		committee, err := committee.NewMainConsensusCommitteeState(ps, participant.NodeID, selection)
		if err != nil {
			return nil, nil, err
		}

		// create signer
		// TODO: The DKG data is now included in the epoch commit event, which is in turn included in the root seal. If
		// we want to properly sign the block QC, we will have to untangle all of this logic here.
		stakingSigner := signature.NewAggregationProvider(encoding.ConsensusVoteTag, local)
		beaconSigner := signature.NewThresholdProvider(encoding.RandomBeaconTag, participant.RandomBeaconPrivKey)
		merger := signature.NewCombiner()
		signer := verification.NewCombinedSigner(committee, stakingSigner, beaconSigner, merger, participant.NodeID)
		signers[i] = signer

		// create validator
		v := validator.New(committee, forks, signer)
		validators[i] = v
	}

	return validators, signers, nil
}

func NewProtocolState(block *flow.Block) (*protoBadger.State, *badger.DB, error) {

	dir, err := tempDBDir()
	if err != nil {
		return nil, nil, err
	}

	opts := badger.
		DefaultOptions(dir).
		WithKeepL0InMemory(true).
		WithLogger(nil)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, nil, err
	}

	metrics := metrics.NewNoopCollector()

	headers := storeBadger.NewHeaders(metrics, db)
	guarantees := storeBadger.NewGuarantees(metrics, db)
	seals := storeBadger.NewSeals(metrics, db)
	index := storeBadger.NewIndex(metrics, db)
	payloads := storeBadger.NewPayloads(db, index, guarantees, seals)
	blocks := storeBadger.NewBlocks(db, headers, payloads)

	state, err := protoBadger.NewState(metrics, db, headers, seals, index, payloads, blocks)
	if err != nil {
		return nil, nil, err
	}

	result := bootstrap.Result(block, unittest.GenesisStateCommitment)
	seal := bootstrap.Seal(result)
	err = state.Mutate().Bootstrap(block, result, seal)
	if err != nil {
		return nil, nil, err
	}

	return state, db, err
}

func tempDBDir() (string, error) {
	return ioutil.TempDir("", "flow-bootstrap-db")
}

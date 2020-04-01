package run

import (
	"fmt"
	"io/ioutil"

	"github.com/dgraph-io/badger/v2"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/verification"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/viewstate"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module/signature"
	"github.com/dapperlabs/flow-go/state/dkg"
	"github.com/dapperlabs/flow-go/state/protocol"
	protoBadger "github.com/dapperlabs/flow-go/state/protocol/badger"
)

type Signer struct {
	Identity            *flow.Identity
	StakingPrivKey      crypto.PrivateKey
	RandomBeaconPrivKey crypto.PrivateKey
}

type SignerData struct {
	DKGState dkg.State
	Signers  []Signer
}

func GenerateGenesisQC(signerData SignerData, block *flow.Block) (*model.QuorumCertificate, error) {
	ps, db, err := NewProtocolState(block)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	validators, signers, err := createValidators(ps, signerData, block)
	if err != nil {
		return nil, err
	}

	hotBlock := model.Block{
		BlockID:     block.ID(),
		View:        block.View,
		ProposerID:  block.ProposerID,
		QC:          nil,
		PayloadHash: block.PayloadHash,
		Timestamp:   block.Timestamp,
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

func createValidators(ps protocol.State, signerData SignerData, block *flow.Block) ([]hotstuff.Validator, []hotstuff.Signer, error) {
	n := len(signerData.Signers)

	groupSize, err := signerData.DKGState.GroupSize()
	if err != nil {
		return nil, nil, fmt.Errorf("could not get DKG group size: %w", err)
	}
	if groupSize < uint(n) {
		return nil, nil, fmt.Errorf("need at least as many signers as DKG participants, got %v and %v", groupSize, n)
	}

	providers := make([]hotstuff.Signer, n)
	validators := make([]hotstuff.Validator, n)

	f := &mocks.ForksReader{}

	for i, signer := range signerData.Signers {
		// create signer
		stakingSigner := signature.NewAggregationProvider("staking_tag", signer.StakingPrivKey)
		beaconSigner := signature.NewThresholdProvider("beacon_tag", signer.RandomBeaconPrivKey)
		merger := signature.NewCombiner()
		selector := filter.And(filter.HasRole(flow.RoleConsensus), filter.HasStake(true))
		provider := verification.NewCombinedSigner(ps, signerData.DKGState, stakingSigner, beaconSigner, merger, selector, signer.Identity.NodeID)
		providers[i] = provider

		// create view state
		vs, err := viewstate.New(ps, signerData.DKGState, signer.Identity.NodeID, selector)
		if err != nil {
			return nil, nil, err
		}

		// create validator
		v := validator.New(vs, f, provider)
		validators[i] = v
	}

	return validators, providers, nil
}

func NewProtocolState(block *flow.Block) (*protoBadger.State, *badger.DB, error) {
	dir, err := tempDBDir()
	if err != nil {
		return nil, nil, err
	}

	db, err := badger.Open(badger.DefaultOptions(dir).WithLogger(nil))
	if err != nil {
		return nil, nil, err
	}

	state, err := protoBadger.NewState(db)
	if err != nil {
		return nil, nil, err
	}

	err = state.Mutate().Bootstrap(block)
	if err != nil {
		return nil, nil, err
	}

	return state, db, err
}

func tempDBDir() (string, error) {
	return ioutil.TempDir("", "flow-bootstrap-db")
}

package cmd

import (
	"encoding/binary"
	"encoding/hex"
	"math/rand"

	"github.com/onflow/flow-go/cmd/bootstrap/run"
	"github.com/onflow/flow-go/consensus/hotstuff/committees/leader"
	model "github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
)

func constructRootResultAndSeal(
	rootCommit string,
	block *flow.Block,
	participantNodes []model.NodeInfo,
	assignments flow.AssignmentList,
	clusterQCs []*flow.QuorumCertificate,
	dkgData model.DKGData,
) (*flow.ExecutionResult, *flow.Seal) {

	stateCommit, err := hex.DecodeString(rootCommit)
	if err != nil {
		log.Fatal().Err(err).Msg("could not decode state commitment")
	}

	participants := model.ToIdentityList(participantNodes)

	epochSetup := &flow.EpochSetup{
		Counter:      flagEpochCounter,
		FirstView:    block.Header.View,
		FinalView:    block.Header.View + leader.EstimatedSixMonthOfViews,
		Participants: participants,
		Assignments:  assignments,
		RandomSource: getRandomSource(block.ID()),
	}

	dkgLookup := model.ToDKGLookup(dkgData, participants)

	epochCommit := &flow.EpochCommit{
		Counter:         flagEpochCounter,
		ClusterQCs:      clusterQCs,
		DKGGroupKey:     dkgData.PubGroupKey,
		DKGParticipants: dkgLookup,
	}

	result := run.GenerateRootResult(block, stateCommit, epochSetup, epochCommit)
	seal := run.GenerateRootSeal(result)

	return result, seal
}

// getRandomSource produces the random source which is included in the root
// EpochSetup event and is used for leader selection. The random source is
// generated deterministically based on the root block ID, so that the
// bootstrapping files are deterministic across runs for the same inputs.
func getRandomSource(rootBlockID flow.Identifier) []byte {

	seed := int64(binary.BigEndian.Uint64(rootBlockID[:]))
	rng := rand.New(rand.NewSource(seed))
	randomSource := make([]byte, flow.EpochSetupRandomSourceLength)
	_, err := rng.Read(randomSource)
	if err != nil {
		log.Fatal().Err(err).Msg("could not generate random source for epoch setup event")
	}

	return randomSource
}

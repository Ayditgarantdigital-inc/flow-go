package verification

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/helper"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/signature"
	"github.com/dapperlabs/flow-go/state/dkg"
	dkgmock "github.com/dapperlabs/flow-go/state/dkg/mocks"
	"github.com/dapperlabs/flow-go/state/protocol"
	protomock "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func MakeSigners(t *testing.T, proto protocol.State, dkg dkg.State, signerIDs []flow.Identifier, stakingKeys []crypto.PrivateKey, beaconKeys []crypto.PrivateKey) []hotstuff.Signer {

	// generate our consensus node identities
	require.NotEmpty(t, signerIDs)

	var signers []hotstuff.Signer
	if len(beaconKeys) != len(stakingKeys) {
		for i, signerID := range signerIDs {
			signer := MakeStakingSigner(t, proto, signerID, stakingKeys[i])
			signers = append(signers, signer)
		}
	} else {
		for i, signerID := range signerIDs {
			signer := MakeBeaconSigner(t, proto, dkg, signerID, stakingKeys[i], beaconKeys[i])
			signers = append(signers, signer)
		}
	}

	return signers
}

func MakeStakingSigner(t *testing.T, state protocol.State, signerID flow.Identifier, priv crypto.PrivateKey) *SingleSigner {
	local, err := local.New(nil, priv)
	require.NoError(t, err)
	staking := signature.NewAggregationProvider("test_staking", local)
	signer := NewSingleSigner(state, staking, filter.Any, signerID)
	return signer
}

func MakeBeaconSigner(t *testing.T, proto protocol.State, dkg dkg.State, signerID flow.Identifier, stakingPriv crypto.PrivateKey, beaconPriv crypto.PrivateKey) *CombinedSigner {
	local, err := local.New(nil, stakingPriv)
	require.NoError(t, err)
	staking := signature.NewAggregationProvider("test_staking", local)
	beacon := signature.NewThresholdProvider("test_beacon", beaconPriv)
	combiner := signature.NewCombiner()
	signer := NewCombinedSigner(proto, dkg, staking, beacon, combiner, filter.Any, signerID)
	return signer
}

func MakeProtocolState(t *testing.T, identities flow.IdentityList, beaconEnabled bool) (protocol.State, dkg.State, []crypto.PrivateKey, []crypto.PrivateKey) {

	// initialize the dkg snapshot
	dkg := &dkgmock.State{}

	// program the state snapshot
	snapshot := &protomock.Snapshot{}
	snapshot.On("Identities", mock.Anything).Return(func(selector flow.IdentityFilter) flow.IdentityList {
		return identities.Filter(selector)
	}, nil)
	for _, identity := range identities {
		snapshot.On("Identity", identity.NodeID).Return(identity, nil)
	}

	// generate the staking keys
	var stakingKeys []crypto.PrivateKey
	for i := 0; i < len(identities); i++ {
		stakingKey := helper.MakeBLSKey(t)
		identities[i].StakingPubKey = stakingKey.PublicKey()
		stakingKeys = append(stakingKeys, stakingKey)
	}

	// generate the dkg keys (only if becon is enabled
	var beaconKeys []crypto.PrivateKey
	var dkgKey crypto.PublicKey
	if beaconEnabled {
		beaconKeys, dkgKey, _ = unittest.RunDKG(t, len(identities))
		dkg.On("GroupSize").Return(uint(len(beaconKeys)), nil)
		dkg.On("GroupKey").Return(dkgKey, nil)
		for i, identity := range identities {
			dkg.On("HasParticipant", identity.NodeID).Return(true, nil)
			dkg.On("ParticipantIndex", identity.NodeID).Return(uint(i), nil)
			dkg.On("ParticipantKey", identity.NodeID).Return(beaconKeys[i].PublicKey(), nil)
		}
	}

	// program the protocol state
	state := &protomock.State{}
	state.On("AtBlockID", mock.Anything).Return(snapshot)
	state.On("Final").Return(snapshot)

	return state, dkg, stakingKeys, beaconKeys
}

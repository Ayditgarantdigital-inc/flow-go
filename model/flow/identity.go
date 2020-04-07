package flow

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"regexp"
	"sort"
	"strconv"

	"github.com/pkg/errors"

	"github.com/dapperlabs/flow-go/crypto"
)

// rxid is the regex for parsing node identity entries.
var rxid = regexp.MustCompile(`^(collection|consensus|execution|verification|observation)-([0-9a-fA-F]{64})@([\w\d]+|[\w\d][\w\d\-]*[\w\d](?:\.*[\w\d][\w\d\-]*[\w\d])*|[\w\d][\w\d\-]*[\w\d])(:[\d]+)?=(\d{1,20})$`)

// Identity represents a node identity.
type Identity struct {
	NodeID        Identifier
	Address       string
	Role          Role
	Stake         uint64
	StakingPubKey crypto.PublicKey
	NetworkPubKey crypto.PublicKey
}

// ParseIdentity parses a string representation of an identity.
func ParseIdentity(identity string) (*Identity, error) {

	// use the regex to match the four parts of an identity
	matches := rxid.FindStringSubmatch(identity)
	if len(matches) != 6 {
		return nil, errors.New("invalid identity string format")
	}

	// none of these will error as they are checked by the regex
	var nodeID Identifier
	nodeID, err := HexStringToIdentifier(matches[2])
	if err != nil {
		return nil, err
	}
	address := matches[3] + matches[4]
	role, _ := ParseRole(matches[1])
	stake, _ := strconv.ParseUint(matches[5], 10, 64)

	// create the identity
	iy := Identity{
		NodeID:  nodeID,
		Address: address,
		Role:    role,
		Stake:   stake,
	}

	return &iy, nil
}

// String returns a string representation of the identity.
func (iy Identity) String() string {
	return fmt.Sprintf("%s-%s@%s=%d", iy.Role, iy.NodeID.String(), iy.Address, iy.Stake)
}

// ID returns a unique identifier for the identity.
func (iy Identity) ID() Identifier {
	return iy.NodeID
}

// Checksum returns a checksum for the identity including mutable attributes.
func (iy Identity) Checksum() Identifier {
	return MakeID(iy)
}

type jsonMarshalIdentity struct {
	NodeID        Identifier
	Address       string
	Role          Role
	Stake         uint64
	StakingPubKey []byte
	NetworkPubKey []byte
}

func (iy *Identity) UnmarshalJSON(b []byte) error {
	var m jsonMarshalIdentity
	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}
	iy.NodeID = m.NodeID
	iy.Address = m.Address
	iy.Role = m.Role
	iy.Stake = m.Stake
	var err error
	if m.StakingPubKey != nil {
		if iy.StakingPubKey, err = crypto.DecodePublicKey(crypto.BLS_BLS12381, m.StakingPubKey); err != nil {
			return err
		}
	}
	if m.NetworkPubKey != nil {
		if iy.NetworkPubKey, err = crypto.DecodePublicKey(crypto.ECDSA_P256, m.NetworkPubKey); err != nil {
			return err
		}
	}
	return nil
}

func (iy Identity) MarshalJSON() ([]byte, error) {
	m := jsonMarshalIdentity{iy.NodeID, iy.Address, iy.Role, iy.Stake, nil, nil}
	var err error
	if iy.StakingPubKey != nil {
		if m.StakingPubKey, err = iy.StakingPubKey.Encode(); err != nil {
			return nil, err
		}
	}
	if iy.NetworkPubKey != nil {
		if m.NetworkPubKey, err = iy.NetworkPubKey.Encode(); err != nil {
			return nil, err
		}
	}

	return json.Marshal(m)
}

// IdentityFilter is a filter on identities.
type IdentityFilter func(*Identity) bool

// IdentityOrder is a sort for identities.
type IdentityOrder func(*Identity, *Identity) bool

// IdentityList is a list of nodes.
type IdentityList []*Identity

// Filter will apply a filter to the identity list.
func (il IdentityList) Filter(filters ...IdentityFilter) IdentityList {
	var dup IdentityList
IDLoop:
	for _, identity := range il {
		for _, filter := range filters {
			if !filter(identity) {
				continue IDLoop
			}
		}
		dup = append(dup, identity)
	}
	return dup
}

// Order will sort the list using the given sort function.
func (il IdentityList) Order(less IdentityOrder) IdentityList {
	dup := make(IdentityList, 0, len(il))
	dup = append(dup, il...)
	sort.Slice(dup, func(i int, j int) bool {
		return less(dup[i], dup[j])
	})
	return dup
}

// NodeIDs returns the NodeIDs of the nodes in the list.
func (il IdentityList) NodeIDs() []Identifier {
	nodeIDs := make([]Identifier, 0, len(il))
	for _, id := range il {
		nodeIDs = append(nodeIDs, id.NodeID)
	}
	return nodeIDs
}

func (il IdentityList) Fingerprint() Identifier {
	return MerkleRoot(GetIDs(il)...)
}

// TotalStake returns the total stake of all given identities.
func (il IdentityList) TotalStake() uint64 {
	var total uint64
	for _, identity := range il {
		total += identity.Stake
	}
	return total
}

// Count returns the count of identities.
func (il IdentityList) Count() uint {
	return uint(len(il))
}

// ByIndex returns the node at the given index.
func (il IdentityList) ByIndex(index uint) (*Identity, bool) {
	if index >= uint(len(il)) {
		return nil, false
	}
	return il[int(index)], true
}

// ByNodeID gets a node from the list by node ID.
func (il IdentityList) ByNodeID(nodeID Identifier) (*Identity, bool) {
	for _, identity := range il {
		if identity.NodeID == nodeID {
			return identity, true
		}
	}
	return nil, false
}

// Sample returns simple random sample from the `IdentityList`
func (il IdentityList) Sample(size uint) IdentityList {
	if size > uint(len(il)) {
		size = uint(len(il))
	}
	dup := make([]*Identity, 0, len(il))
	for _, identity := range il {
		dup = append(dup, identity)
	}
	rand.Shuffle(len(dup), func(i int, j int) {
		dup[i], dup[j] = dup[j], dup[i]
	})
	return dup[:size]
}

// StakingKeys returns a list of the staking public keys for the identities.
func (il IdentityList) StakingKeys() []crypto.PublicKey {
	keys := make([]crypto.PublicKey, 0, len(il))
	for _, identity := range il {
		keys = append(keys, identity.StakingPubKey)
	}
	return keys
}

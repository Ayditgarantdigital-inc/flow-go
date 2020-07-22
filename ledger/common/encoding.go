package common

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/dapperlabs/flow-go/ledger"
)

// EncodingDecodingVersion encoder/decoder code only supports
// decoding data with version smaller or equal to this value
// bumping this number prevents older versions of the code
// to deal with the newer version of data
// codes should be updated with backward compatibility if needed
// and act differently based on the encoding decoding version
const EncodingDecodingVersion = uint16(0)

// EncodingType capture the type of encoded entity
type EncodingType uint8

const (
	// EncodingTypeStateCommitment - encoding type for StateCommitments
	EncodingTypeStateCommitment = iota
	// EncodingTypeKeyPart - encoding type for KeyParts (a subset of key)
	EncodingTypeKeyPart
	// EncodingTypeKey - encoding type for Keys (unique identifier to reference a location in ledger)
	EncodingTypeKey
	// EncodingTypeValue - encoding type for Ledger Values
	EncodingTypeValue
	// EncodingTypePath - encoding type for Paths (trie storage location of a key value pair)
	EncodingTypePath
	// EncodingTypePayload - encoding type for Payloads (stored at trie nodes including key value pair )
	EncodingTypePayload
	// EncodingTypeProof encoding type for Proofs
	// (all data needed to verify a key value pair at specific stateCommitment)
	EncodingTypeProof
	// EncodingTypeBatchProof - encoding type for BatchProofs
	EncodingTypeBatchProof
	// EncodingTypeQuery - encoding type for ledger query
	EncodingTypeQuery
	// EncodingTypeUpdate - encoding type for ledger update
	EncodingTypeUpdate
	// EncodingTypeTrieUpdate - encoding type for trie update
	EncodingTypeTrieUpdate
	// encodingTypeUnknown - unknown encoding type - Warning this should always be the last item in the list
	encodingTypeUnknown
)

func (e EncodingType) String() string {
	return [...]string{"StateCommitment", "KeyPart", "Key", "Value", "Path", "Payload", "Proof", "BatchProof", "Unknown"}[e]
}

// CheckEncDecVer extracts encoding bytes from a raw encoded message
// checks it against the supported versions and returns the rest of rawInput (excluding encDecVersion bytes)
func CheckEncDecVer(rawInput []byte) (rest []byte, version uint16, err error) {
	version, rest, err = ReadUint16(rawInput)
	if err != nil {
		return rest, version, fmt.Errorf("error checking the encoding decoding version: %w", err)
	}
	// error on versions coming from future till a time-machine is invented
	if version > EncodingDecodingVersion {
		return rest, version, fmt.Errorf("incompatible encoding decoding version (%d > %d): %w", version, EncodingDecodingVersion, err)
	}
	// return the rest of bytes
	return rest, version, nil
}

// CheckEncodingType extracts encoding byte from a raw encoded message
// checks it against the supported versions and returns the rest of rawInput (excluding encDecVersion bytes)
func CheckEncodingType(rawInput []byte, expectedType uint8) (rest []byte, err error) {
	t, r, err := ReadUint8(rawInput)
	if err != nil {
		return r, fmt.Errorf("error checking type of the encoded entity: %w", err)
	}

	// error if type is known for this code
	if t >= encodingTypeUnknown {
		return r, fmt.Errorf("unknown entity type in the encoded data (%d > %d)", t, encodingTypeUnknown)
	}

	// error if type is known for this code
	if t != expectedType {
		return r, fmt.Errorf("unexpected entity type, got (%v) but (%v) was expected", EncodingType(t), EncodingType(expectedType))
	}

	// return the rest of bytes
	return r, nil
}

// EncodeKeyPart encodes a key part into a byte slice
func EncodeKeyPart(kp *ledger.KeyPart) []byte {
	if kp == nil {
		return []byte{}
	}
	// EncodingDecodingType
	buffer := AppendUint16([]byte{}, EncodingDecodingVersion)

	// encode key part entity type
	buffer = AppendUint8(buffer, EncodingTypeKeyPart)

	// encode the key part content
	buffer = append(buffer, encodeKeyPart(kp)...)
	return buffer
}

func encodeKeyPart(kp *ledger.KeyPart) []byte {
	buffer := make([]byte, 0)

	// encode "Type" field of the key part
	buffer = AppendUint16(buffer, kp.Type)

	// encode "Value" field of the key part
	buffer = append(buffer, kp.Value...)
	return buffer
}

// DecodeKeyPart constructs a key part from an encoded key part
func DecodeKeyPart(encodedKeyPart []byte) (*ledger.KeyPart, error) {
	// currently we ignore the version but in the future we
	// can do switch case based on the version if needed
	rest, _, err := CheckEncDecVer(encodedKeyPart)
	if err != nil {
		return nil, fmt.Errorf("error decoding key part: %w", err)
	}

	// check the encoding type
	rest, err = CheckEncodingType(rest, EncodingTypeKeyPart)
	if err != nil {
		return nil, fmt.Errorf("error decoding key part: %w", err)
	}

	// decode the key part content
	key, err := decodeKeyPart(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding key part: %w", err)
	}

	return key, nil
}

func decodeKeyPart(inp []byte) (*ledger.KeyPart, error) {
	// read key part type and the rest is the key item part
	kpt, kpv, err := ReadUint16(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding key part (content): %w", err)
	}
	return &ledger.KeyPart{Type: kpt, Value: kpv}, nil
}

// EncodeKey encodes a key into a byte slice
func EncodeKey(k *ledger.Key) []byte {
	if k == nil {
		return []byte{}
	}
	// encode EncodingDecodingType
	buffer := AppendUint16([]byte{}, EncodingDecodingVersion)
	// encode key entity type
	buffer = AppendUint8(buffer, EncodingTypeKey)
	// encode key content
	buffer = append(buffer, encodeKey(k)...)

	return buffer
}

// encodeKey encodes a key into a byte slice
func encodeKey(k *ledger.Key) []byte {
	buffer := make([]byte, 0)
	// encode number of key parts
	buffer = AppendUint16(buffer, uint16(len(k.KeyParts)))
	// iterate over key parts
	for _, kp := range k.KeyParts {
		// encode the key part
		encKP := encodeKeyPart(&kp)
		// encode the len of the encoded key part
		buffer = AppendUint32(buffer, uint32(len(encKP)))
		// append the encoded key part
		buffer = append(buffer, encKP...)
	}
	return buffer
}

// DecodeKey constructs a key from an encoded key part
func DecodeKey(encodedKey []byte) (*ledger.Key, error) {
	// check the enc dec version
	rest, _, err := CheckEncDecVer(encodedKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %w", err)
	}
	// check the encoding type
	rest, err = CheckEncodingType(rest, EncodingTypeKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %w", err)
	}

	// decode the key content
	key, err := decodeKey(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding key: %w", err)
	}
	return key, nil
}

func decodeKey(inp []byte) (*ledger.Key, error) {
	key := &ledger.Key{}
	numOfParts, rest, err := ReadUint16(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding key (content): %w", err)
	}

	for i := 0; i < int(numOfParts); i++ {
		var kpEncSize uint32
		var kpEnc []byte
		// read encoded key part size
		kpEncSize, rest, err = ReadUint32(rest)
		if err != nil {
			return nil, fmt.Errorf("error decoding key (content): %w", err)
		}

		// read encoded key part
		kpEnc, rest, err = ReadSlice(rest, int(kpEncSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding key (content): %w", err)
		}

		// decode encoded key part
		kp, err := decodeKeyPart(kpEnc)
		if err != nil {
			return nil, fmt.Errorf("error decoding key (content): %w", err)
		}
		key.KeyParts = append(key.KeyParts, *kp)
	}
	return key, nil
}

// EncodeValue encodes a value into a byte slice
func EncodeValue(v ledger.Value) []byte {
	// encode EncodingDecodingType
	buffer := AppendUint16([]byte{}, EncodingDecodingVersion)

	// encode key entity type
	buffer = AppendUint8(buffer, EncodingTypeValue)

	// encode value
	buffer = append(buffer, encodeValue(v)...)

	return buffer
}

func encodeValue(v ledger.Value) []byte {
	return v
}

// DecodeValue constructs a ledger value using an encoded byte slice
func DecodeValue(encodedValue []byte) (ledger.Value, error) {
	// check enc dec version
	rest, _, err := CheckEncDecVer(encodedValue)
	if err != nil {
		return nil, err
	}

	// check the encoding type
	rest, err = CheckEncodingType(rest, EncodingTypeValue)
	if err != nil {
		return nil, err
	}

	return decodeValue(rest)
}

func decodeValue(inp []byte) (ledger.Value, error) {
	return ledger.Value(inp), nil
}

// EncodePath encodes a path into a byte slice
func EncodePath(p ledger.Path) []byte {
	// encode EncodingDecodingType
	buffer := AppendUint16([]byte{}, EncodingDecodingVersion)

	// encode key entity type
	buffer = AppendUint8(buffer, EncodingTypePath)

	// encode path
	buffer = append(buffer, encodePath(p)...)

	return buffer
}

func encodePath(p ledger.Path) []byte {
	return p
}

// DecodePath constructs a path value using an encoded byte slice
func DecodePath(encodedPath []byte) (ledger.Path, error) {
	// check enc dec version
	rest, _, err := CheckEncDecVer(encodedPath)
	if err != nil {
		return nil, err
	}

	// check the encoding type
	rest, err = CheckEncodingType(rest, EncodingTypePath)
	if err != nil {
		return nil, err
	}

	return decodePath(rest)
}

func decodePath(inp []byte) (ledger.Path, error) {
	return ledger.Path(inp), nil
}

// EncodePayload encodes a ledger payload
func EncodePayload(p *ledger.Payload) []byte {
	if p == nil {
		return []byte{}
	}
	// encode EncodingDecodingType
	buffer := AppendUint16([]byte{}, EncodingDecodingVersion)

	// encode key entity type
	buffer = AppendUint8(buffer, EncodingTypePayload)

	// append encoded payload content
	buffer = append(buffer, encodePayload(p)...)

	return buffer
}

func encodePayload(p *ledger.Payload) []byte {
	buffer := make([]byte, 0)

	// encode key
	encK := encodeKey(&p.Key)

	// encode encoded key size
	buffer = AppendUint32(buffer, uint32(len(encK)))

	// append encoded key content
	buffer = append(buffer, encK...)

	// encode value
	encV := encodeValue(p.Value)

	// encode encoded value size
	buffer = AppendUint64(buffer, uint64(len(encV)))

	// append encoded key content
	buffer = append(buffer, encV...)

	return buffer
}

// DecodePayload construct a payload from an encoded byte slice
func DecodePayload(encodedPayload []byte) (*ledger.Payload, error) {
	// if empty don't decode
	if len(encodedPayload) == 0 {
		return nil, nil
	}
	// check the enc dec version
	rest, _, err := CheckEncDecVer(encodedPayload)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}
	// check the encoding type
	rest, err = CheckEncodingType(rest, EncodingTypePayload)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}
	return decodePayload(rest)
}

func decodePayload(inp []byte) (*ledger.Payload, error) {

	// read encoded key size
	encKeySize, rest, err := ReadUint32(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// read encoded key
	encKey, rest, err := ReadSlice(rest, int(encKeySize))
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// decode the key
	key, err := decodeKey(encKey)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// read encoded value size
	encValeSize, rest, err := ReadUint64(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// read encoded value
	encValue, _, err := ReadSlice(rest, int(encValeSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	// decode value
	value, err := decodeValue(encValue)
	if err != nil {
		return nil, fmt.Errorf("error decoding payload: %w", err)
	}

	return &ledger.Payload{Key: *key, Value: value}, nil
}

// EncodeTrieUpdate encodes a trie update struct
func EncodeTrieUpdate(t *ledger.TrieUpdate) []byte {
	if t == nil {
		return []byte{}
	}
	// encode EncodingDecodingType
	buffer := AppendUint16([]byte{}, EncodingDecodingVersion)

	// encode key entity type
	buffer = AppendUint8(buffer, EncodingTypeTrieUpdate)

	// append encoded payload content
	buffer = append(buffer, encodeTrieUpdate(t)...)

	return buffer
}

func encodeTrieUpdate(t *ledger.TrieUpdate) []byte {
	buffer := make([]byte, 0)

	// encode root hash (size and data)
	buffer = AppendUint16(buffer, uint16(len(t.RootHash)))
	buffer = append(buffer, t.RootHash...)

	// encode number of paths
	buffer = AppendUint32(buffer, uint32(t.Size()))

	if t.Size() == 0 {
		return buffer
	}

	// encode paths
	// encode path size (assuming all paths are the same size)
	buffer = AppendUint16(buffer, uint16(t.Paths[0].Size()))
	for _, path := range t.Paths {
		buffer = append(buffer, encodePath(path)...)
	}

	// we assume same number of payloads
	// encode payloads
	for _, pl := range t.Payloads {
		encPl := encodePayload(pl)
		buffer = AppendUint32(buffer, uint32(len(encPl)))
		buffer = append(buffer, encPl...)
	}

	return buffer
}

// DecodeTrieUpdate construct a trie update from an encoded byte slice
func DecodeTrieUpdate(encodedTrieUpdate []byte) (*ledger.TrieUpdate, error) {
	// if empty don't decode
	if len(encodedTrieUpdate) == 0 {
		return nil, nil
	}
	// check the enc dec version
	rest, _, err := CheckEncDecVer(encodedTrieUpdate)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}
	// check the encoding type
	rest, err = CheckEncodingType(rest, EncodingTypeTrieUpdate)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}
	return decodeTrieUpdate(rest)
}

func decodeTrieUpdate(inp []byte) (*ledger.TrieUpdate, error) {

	paths := make([]ledger.Path, 0)
	payloads := make([]*ledger.Payload, 0)

	// decode root hash
	rhSize, rest, err := ReadUint16(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}

	rh, rest, err := ReadSlice(rest, int(rhSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}

	// decode number of paths
	numOfPaths, rest, err := ReadUint32(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}

	// decode path size
	pathSize, rest, err := ReadUint16(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding trie update: %w", err)
	}

	var path ledger.Path
	var encPath []byte
	for i := 0; i < int(numOfPaths); i++ {
		encPath, rest, err = ReadSlice(rest, int(pathSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		path, err = decodePath(encPath)
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		paths = append(paths, path)
	}

	var payloadSize uint32
	var encPayload []byte
	var payload *ledger.Payload

	for i := 0; i < int(numOfPaths); i++ {
		payloadSize, rest, err = ReadUint32(rest)
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		encPayload, rest, err = ReadSlice(rest, int(payloadSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		payload, err = decodePayload(encPayload)
		if err != nil {
			return nil, fmt.Errorf("error decoding trie update: %w", err)
		}
		payloads = append(payloads, payload)
	}
	return &ledger.TrieUpdate{RootHash: rh, Paths: paths, Payloads: payloads}, nil
}

// EncodeProof encodes the content of a proof into a byte slice
func EncodeTrieProof(p *ledger.TrieProof) []byte {
	if p == nil {
		return []byte{}
	}
	// encode EncodingDecodingType
	buffer := AppendUint16([]byte{}, EncodingDecodingVersion)

	// encode key entity type
	buffer = AppendUint8(buffer, EncodingTypeProof)

	// append encoded proof content
	buffer = append(buffer, encodeTrieProof(p)...)

	return buffer
}

func encodeTrieProof(p *ledger.TrieProof) []byte {
	// first byte is reserved for inclusion flag
	buffer := make([]byte, 1)
	if p.Inclusion {
		// set the first bit to 1 if it is an inclusion proof
		buffer[0] |= 1 << 7
	}

	// steps are encoded as a single byte
	buffer = AppendUint8(buffer, p.Steps)

	// include flags size and content
	buffer = AppendUint8(buffer, uint8(len(p.Flags)))
	buffer = append(buffer, p.Flags...)

	// include path size and content
	buffer = AppendUint16(buffer, uint16(p.Path.Size()))
	buffer = append(buffer, p.Path...)

	// include encoded payload size and content
	encPayload := encodePayload(p.Payload)
	buffer = AppendUint64(buffer, uint64(len(encPayload)))
	buffer = append(buffer, encPayload...)

	// and finally include all interims (hash values)
	// number of interims
	buffer = AppendUint8(buffer, uint8(len(p.Interims)))
	for _, inter := range p.Interims {
		buffer = AppendUint16(buffer, uint16(len(inter)))
		buffer = append(buffer, inter...)
	}

	return buffer
}

// DecodeProof construct a proof from an encoded byte slice
func DecodeTrieProof(encodedProof []byte) (*ledger.TrieProof, error) {
	// check the enc dec version
	rest, _, err := CheckEncDecVer(encodedProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	// check the encoding type
	rest, err = CheckEncodingType(rest, EncodingTypeProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	return decodeTrieProof(rest)
}

func decodeTrieProof(inp []byte) (*ledger.TrieProof, error) {
	pInst := ledger.NewTrieProof()

	// Inclusion flag
	byteInclusion, rest, err := ReadSlice(inp, 1)
	pInst.Inclusion, _ = IsBitSet(byteInclusion, 0)

	// read steps
	steps, rest, err := ReadUint8(rest)
	pInst.Steps = steps

	// read flags
	flagsSize, rest, err := ReadUint8(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	flags, rest, err := ReadSlice(rest, int(flagsSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	pInst.Flags = flags

	// read path
	pathSize, rest, err := ReadUint16(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	path, rest, err := ReadSlice(rest, int(pathSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	pInst.Path = path

	// read payload
	encPayloadSize, rest, err := ReadUint64(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	encPayload, rest, err := ReadSlice(rest, int(encPayloadSize))
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	payload, err := decodePayload(encPayload)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	pInst.Payload = payload

	// read interims
	interimsLen, rest, err := ReadUint8(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding proof: %w", err)
	}
	interims := make([][]byte, 0)

	var interimSize uint16
	var interim []byte

	for i := 0; i < int(interimsLen); i++ {
		interimSize, rest, err = ReadUint16(rest)
		if err != nil {
			return nil, fmt.Errorf("error decoding proof: %w", err)
		}

		interim, rest, err = ReadSlice(rest, int(interimSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding proof: %w", err)
		}
		interims = append(interims, interim)
	}
	pInst.Interims = interims

	return pInst, nil
}

// EncodeTrieBatchProof encodes a batch proof into a byte slice
func EncodeTrieBatchProof(bp *ledger.TrieBatchProof) []byte {
	if bp == nil {
		return []byte{}
	}
	// encode EncodingDecodingType
	buffer := AppendUint16([]byte{}, EncodingDecodingVersion)

	// encode key entity type
	buffer = AppendUint8(buffer, EncodingTypeBatchProof)
	// encode batch proof content
	buffer = append(buffer, encodeTrieBatchProof(bp)...)

	return buffer
}

// encodeBatchProof encodes a batch proof into a byte slice
func encodeTrieBatchProof(bp *ledger.TrieBatchProof) []byte {
	buffer := make([]byte, 0)
	// encode number of proofs
	buffer = AppendUint32(buffer, uint32(len(bp.Proofs)))
	// iterate over proofs
	for _, p := range bp.Proofs {
		// encode the proof
		encP := encodeTrieProof(p)
		// encode the len of the encoded proof
		buffer = AppendUint64(buffer, uint64(len(encP)))
		// append the encoded proof
		buffer = append(buffer, encP...)
	}
	return buffer
}

// DecodeTrieBatchProof constructs a batch proof from an encoded byte slice
func DecodeTrieBatchProof(encodedBatchProof []byte) (*ledger.TrieBatchProof, error) {
	// check the enc dec version
	rest, _, err := CheckEncDecVer(encodedBatchProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof: %w", err)
	}
	// check the encoding type
	rest, err = CheckEncodingType(rest, EncodingTypeBatchProof)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof: %w", err)
	}

	// decode the batch proof content
	bp, err := decodeTrieBatchProof(rest)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof: %w", err)
	}
	return bp, nil
}

func decodeTrieBatchProof(inp []byte) (*ledger.TrieBatchProof, error) {
	bp := ledger.NewTrieBatchProof()
	// number of proofs
	numOfProofs, rest, err := ReadUint32(inp)
	if err != nil {
		return nil, fmt.Errorf("error decoding batch proof (content): %w", err)
	}

	for i := 0; i < int(numOfProofs); i++ {
		var encProofSize uint64
		var encProof []byte
		// read encoded proof size
		encProofSize, rest, err = ReadUint64(rest)
		if err != nil {
			return nil, fmt.Errorf("error decoding batch proof (content): %w", err)
		}

		// read encoded proof
		encProof, rest, err = ReadSlice(rest, int(encProofSize))
		if err != nil {
			return nil, fmt.Errorf("error decoding batch proof (content): %w", err)
		}

		// decode encoded proof
		proof, err := decodeTrieProof(encProof)
		if err != nil {
			return nil, fmt.Errorf("error decoding batch proof (content): %w", err)
		}
		bp.Proofs = append(bp.Proofs, proof)
	}
	return bp, nil
}

// func WriteUint8(buffer []byte, loc int, value uint8) int {
// 	buffer[loc] = byte(value)
// 	return loc + 1
// }

// func WriteUint16(buffer []byte, loc int, value uint16) int {
// 	binary.BigEndian.PutUint16(buffer[loc:], value)
// 	return loc + 2
// }

// func WriteUint32(buffer []byte, loc int, value uint32) int {
// 	binary.BigEndian.PutUint32(buffer[loc:], value)
// 	return loc + 4
// }

// func WriteUint64(buffer []byte, loc int, value uint64) int {
// 	binary.BigEndian.PutUint64(buffer[loc:], value)
// 	return loc + 8
// }

func AppendUint8(input []byte, value uint8) []byte {
	return append(input, byte(value))
}

func AppendUint16(input []byte, value uint16) []byte {
	buffer := make([]byte, 2)
	binary.BigEndian.PutUint16(buffer, value)
	return append(input, buffer...)
}

func AppendUint32(input []byte, value uint32) []byte {
	buffer := make([]byte, 4)
	binary.BigEndian.PutUint32(buffer, value)
	return append(input, buffer...)
}

func AppendUint64(input []byte, value uint64) []byte {
	buffer := make([]byte, 8)
	binary.BigEndian.PutUint64(buffer, value)
	return append(input, buffer...)
}

// AppendShortData appends data shorter than 16kB
func AppendShortData(input []byte, data []byte) []byte {
	if len(data) > math.MaxUint16 {
		panic(fmt.Sprintf("short data too long! %d", len(data)))
	}
	input = AppendUint16(input, uint16(len(data)))
	input = append(input, data...)
	return input
}

// AppendLongData appends data shorter than 32MB
func AppendLongData(input []byte, data []byte) []byte {
	if len(data) > math.MaxUint32 {
		panic(fmt.Sprintf("long data too long! %d", len(data)))
	}
	input = AppendUint32(input, uint32(len(data)))
	input = append(input, data...)
	return input
}

func ReadSlice(input []byte, size int) (value []byte, rest []byte, err error) {
	if len(input) < size {
		return nil, input, fmt.Errorf("input size is too small to be splited %d < %d ", len(input), size)
	}
	return input[:size], input[size:], nil
}

func ReadUint8(input []byte) (value uint8, rest []byte, err error) {
	if len(input) < 1 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint8", len(input))
	}
	return uint8(input[0]), input[1:], nil
}

func ReadUint16(input []byte) (value uint16, rest []byte, err error) {
	if len(input) < 2 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint16", len(input))
	}
	return binary.BigEndian.Uint16(input[:2]), input[2:], nil
}

func ReadUint32(input []byte) (value uint32, rest []byte, err error) {
	if len(input) < 4 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint32", len(input))
	}
	return binary.BigEndian.Uint32(input[:4]), input[4:], nil
}

func ReadUint64(input []byte) (value uint64, rest []byte, err error) {
	if len(input) < 8 {
		return 0, input, fmt.Errorf("input size (%d) is too small to read a uint64", len(input))
	}
	return binary.BigEndian.Uint64(input[:8]), input[8:], nil
}

// ReadShortData read data shorter than 16kB and return the rest of bytes
func ReadShortData(input []byte) (data []byte, rest []byte, err error) {
	var size uint16
	size, rest, err = ReadUint16(input)
	if err != nil {
		return nil, rest, err
	}
	data = rest[:size]
	return
}

// ReadLongData read data shorter than 32MB and return the rest of bytes
func ReadLongData(input []byte) (data []byte, rest []byte, err error) {
	var size uint32
	size, rest, err = ReadUint32(input)
	if err != nil {
		return nil, rest, err
	}
	data = rest[:size]
	return
}

// ReadShortDataFromReader reads data shorter than 16kB from reader
func ReadShortDataFromReader(reader io.Reader) ([]byte, error) {
	buf, err := ReadFromBuffer(reader, 2)
	if err != nil {
		return nil, fmt.Errorf("cannot read short data length: %w", err)
	}

	size, _, err := ReadUint16(buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read short data length: %w", err)
	}

	buf, err = ReadFromBuffer(reader, int(size))
	if err != nil {
		return nil, fmt.Errorf("cannot read short data: %w", err)
	}

	return buf, nil
}

// ReadLongDataFromReader reads data shorter than 16kB from reader
func ReadLongDataFromReader(reader io.Reader) ([]byte, error) {
	buf, err := ReadFromBuffer(reader, 4)
	if err != nil {
		return nil, fmt.Errorf("cannot read long data length: %w", err)
	}
	size, _, err := ReadUint32(buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read long data length: %w", err)
	}
	buf, err = ReadFromBuffer(reader, int(size))
	if err != nil {
		return nil, fmt.Errorf("cannot read long data: %w", err)
	}

	return buf, nil
}

func ReadFromBuffer(reader io.Reader, length int) ([]byte, error) {
	if length == 0 {
		return nil, nil
	}
	buf := make([]byte, length)
	_, err := io.ReadFull(reader, buf)
	if err != nil {
		return nil, fmt.Errorf("cannot read data: %w", err)
	}
	return buf, nil
}

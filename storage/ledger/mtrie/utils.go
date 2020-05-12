package mtrie

import (
	"fmt"
	"math/rand"
)

// IsBitSet returns if the bit at position i in the byte array b is set to 1 (big endian)
func IsBitSet(b []byte, i int) (bool, error) {
	if i >= len(b)*8 {
		return false, fmt.Errorf("input (%v) only has %d bits, can't look up bit %d", b, len(b)*8, i)
	}
	return b[i/8]&(1<<int(7-i%8)) != 0, nil
}

// SetBit sets the bit at position i in the byte array b to 1
func SetBit(b []byte, i int) error {
	if i >= len(b)*8 {
		return fmt.Errorf("input (%v) only has %d bits, can't set bit %d", b, len(b)*8, i)
	}
	b[i/8] |= 1 << int(7-i%8)
	return nil
}

// SplitKeyValues splits a set of unordered key value pairs based on the value of bit (bitIndex)
func SplitKeyValues(keys [][]byte, values [][]byte, bitIndex int) ([][]byte, [][]byte, [][]byte, [][]byte, error) {

	rkeys := make([][]byte, 0, len(keys))
	rvalues := make([][]byte, 0, len(values))
	lkeys := make([][]byte, 0, len(keys))
	lvalues := make([][]byte, 0, len(values))

	for i, key := range keys {
		bitIsSet, err := IsBitSet(key, bitIndex)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("can't split key values, error: %v", err)
		}
		if bitIsSet {
			rkeys = append(rkeys, key)
			rvalues = append(rvalues, values[i])
		} else {
			lkeys = append(lkeys, key)
			lvalues = append(lvalues, values[i])
		}
	}
	return lkeys, lvalues, rkeys, rvalues, nil
}

// SplitSortedKeys splits a set of ordered keys based on the value of bit (bitIndex)
func SplitSortedKeys(keys [][]byte, bitIndex int) ([][]byte, [][]byte, error) {
	for i, key := range keys {
		bitIsSet, err := IsBitSet(key, bitIndex)
		if err != nil {
			return nil, nil, fmt.Errorf("can't split keys, error: %v", err)
		}
		// found the breaking point
		if bitIsSet {
			return keys[:i], keys[i:], nil
		}
	}
	// all keys have unset bit at bitIndex
	return keys, nil, nil
}

// SplitKeyProofs splits a set of unordered key and proof pairs based on the value of bit (bitIndex)
func SplitKeyProofs(keys [][]byte, proofs []*Proof, bitIndex int) ([][]byte, []*Proof, [][]byte, []*Proof, error) {

	rkeys := make([][]byte, 0, len(keys))
	rproofs := make([]*Proof, 0, len(proofs))
	lkeys := make([][]byte, 0, len(keys))
	lproofs := make([]*Proof, 0, len(proofs))

	for i, key := range keys {
		bitIsSet, err := IsBitSet(key, bitIndex)
		if err != nil {
			return nil, nil, nil, nil, fmt.Errorf("can't split key proof pairs , error: %v", err)
		}
		if bitIsSet {
			rkeys = append(rkeys, key)
			rproofs = append(rproofs, proofs[i])
		} else {
			lkeys = append(lkeys, key)
			lproofs = append(lproofs, proofs[i])
		}
	}
	return lkeys, lproofs, rkeys, rproofs, nil
}

// GetRandomKeysRandN generate m random keys (key size: byteSize),
// the number of keys generates, m, is also randomly selected from the range [1, maxN]
func GetRandomKeysRandN(maxN int, byteSize int) [][]byte {
	numberOfKeys := rand.Intn(maxN) + 1
	return GetRandomKeysFixedN(numberOfKeys, byteSize)
}

// GetRandomKeysFixedN generates n random (no repetition) fixed sized (byteSize) keys
func GetRandomKeysFixedN(n int, byteSize int) [][]byte {
	keys := make([][]byte, 0, n)
	alreadySelectKeys := make(map[string]bool)
	i := 0
	for i < n {
		key := make([]byte, byteSize)
		rand.Read(key)
		// deduplicate
		if _, found := alreadySelectKeys[string(key)]; !found {
			keys = append(keys, key)
			alreadySelectKeys[string(key)] = true
			i++
		}
	}
	return keys
}

// GetRandomValues generate an slice (len n) of
// random values (byte slice) of random size from the range [1, maxByteSize]
func GetRandomValues(n int, maxByteSize int) [][]byte {
	values := make([][]byte, 0, n)
	for i := 0; i < n; i++ {
		byteSize := rand.Intn(maxByteSize) + 1
		value := make([]byte, byteSize)
		rand.Read(value)
		values = append(values, value)
	}
	return values
}

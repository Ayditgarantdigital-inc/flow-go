package signature

import (
	"crypto/rand"
	mathrand "math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
)

// TODO: make tests more robust by testing unhappy paths of splitter

const NUM_SIGS = 10

func TestCombinerJoinSplitEven(t *testing.T) {
	c := &Combiner{}
	sigs := make([]crypto.Signature, 0, NUM_SIGS)
	for i := 0; i < NUM_SIGS; i++ {
		sig := make([]byte, 32)
		n, err := rand.Read(sig[:])
		require.Equal(t, n, len(sig))
		require.NoError(t, err)
		sigs = append(sigs, sig)
	}
	combined, err := c.Join(sigs...)
	require.NoError(t, err)
	splitted, err := c.Split(combined)
	assert.NoError(t, err)
	assert.Equal(t, sigs, splitted)
}

func TestCombinerJoinSplitUneven(t *testing.T) {
	c := &Combiner{}
	sigs := make([]crypto.Signature, 0, NUM_SIGS)
	for i := 0; i < NUM_SIGS; i++ {
		sig := make([]byte, mathrand.Intn(128)+1)
		n, err := rand.Read(sig[:])
		require.Equal(t, n, len(sig))
		require.NoError(t, err)
		sigs = append(sigs, sig)
	}
	combined, err := c.Join(sigs...)
	require.NoError(t, err)
	splitted, err := c.Split(combined)
	assert.NoError(t, err)
	assert.Equal(t, sigs, splitted)
}

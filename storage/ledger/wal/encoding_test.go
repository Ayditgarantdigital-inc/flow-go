package wal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdate(t *testing.T) {

	stateComm := []byte{2, 1, 3, 7}

	keys := [][]byte{
		{1, 2},
		{3, 4},
		{5, 6},
	}

	vals := [][]byte{
		{1},
		{1, 2, 3, 4, 5, 6},
		{1, 2},
	}

	expected := []byte{
		1,    //update flag
		0, 4, //state commit length
		2, 1, 3, 7, //state commit data
		0, 0, 0, 3, //records number
		0, 2, // key size
		1, 2, // first key
		0, 0, 0, 1, //first val len
		1,                                  //first val
		3, 4, 0, 0, 0, 6, 1, 2, 3, 4, 5, 6, //second key + val len + val
		5, 6, 0, 0, 0, 2, 1, 2, //third key + val len + val
	}

	t.Run("size", func(t *testing.T) {
		size := updateSize(stateComm, keys, vals)

		//header + record count + key size + (3 * key expected) + ((4 + 1) + (4 + 6) + (4 + 2)) values with sizes
		assert.Equal(t, 7+4+2+(3*2)+((4+1)+(4+6)+(4+2)), size)

	})

	t.Run("encode", func(t *testing.T) {
		data := EncodeUpdate(stateComm, keys, vals)

		assert.Equal(t, expected, data)
	})

	t.Run("decode", func(t *testing.T) {

		operation, stateCommitment, decodedKeys, decodedValues, err := Decode(expected)
		require.NoError(t, err)

		assert.Equal(t, WALUpdate, operation)
		assert.Equal(t, stateComm, stateCommitment)
		assert.Equal(t, keys, decodedKeys)
		assert.Equal(t, vals, decodedValues)

	})

}

func TestDelete(t *testing.T) {

	stateComm := []byte{2, 1, 3, 7}

	expected := []byte{
		2,    //delete flag
		0, 4, //state commit length
		2, 1, 3, 7, //state commit data
	}

	t.Run("size", func(t *testing.T) {
		size := deleteSize(stateComm)

		// 1 op + 2 state comm size + data
		assert.Equal(t, 7, size)

	})

	t.Run("encode", func(t *testing.T) {
		data := EncodeDelete(stateComm)

		assert.Equal(t, expected, data)
	})

	t.Run("decode", func(t *testing.T) {

		operation, stateCommitment, _, _, err := Decode(expected)
		require.NoError(t, err)

		assert.Equal(t, WALDelete, operation)
		assert.Equal(t, stateComm, stateCommitment)

	})

}

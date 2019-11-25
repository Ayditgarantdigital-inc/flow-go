package checker

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

var storageValueDeclaration = map[string]sema.ValueDeclaration{
	"storage": stdlib.StandardLibraryValue{
		Name:       "storage",
		Type:       &sema.StorageType{},
		Kind:       common.DeclarationKindConstant,
		IsConstant: true,
	},
}

func ParseAndCheckWithStorage(t *testing.T, code string) (*sema.Checker, error) {
	return ParseAndCheckWithOptions(t,
		code,
		ParseAndCheckOptions{
			Options: []sema.Option{
				sema.WithPredeclaredValues(storageValueDeclaration),
				sema.WithAccessCheckMode(sema.AccessCheckModeNotSpecifiedUnrestricted),
			},
		},
	)
}

func TestCheckStorageIndexing(t *testing.T) {

	checker, err := ParseAndCheckWithStorage(t,
		`
          let a = storage[Int]
          let b = storage[Bool]
          let c = storage[[Int]]
          let d = storage[{String: Int}]
        `,
	)

	require.Nil(t, err)

	assert.Equal(t,
		&sema.OptionalType{
			Type: &sema.IntType{},
		},
		checker.GlobalValues["a"].Type,
	)

	assert.Equal(t,
		&sema.OptionalType{
			Type: &sema.BoolType{},
		},
		checker.GlobalValues["b"].Type,
	)

	assert.Equal(t,
		&sema.OptionalType{
			Type: &sema.VariableSizedType{
				Type: &sema.IntType{},
			},
		},
		checker.GlobalValues["c"].Type,
	)

	assert.Equal(t,
		&sema.OptionalType{
			Type: &sema.DictionaryType{
				KeyType:   &sema.StringType{},
				ValueType: &sema.IntType{},
			},
		},
		checker.GlobalValues["d"].Type,
	)
}

func TestCheckStorageIndexingAssignment(t *testing.T) {

	_, err := ParseAndCheckWithStorage(t,
		`
          fun test() {
              storage[Int] = 1
              storage[Bool] = true
          }
        `,
	)

	assert.Nil(t, err)
}

func TestCheckInvalidStorageIndexingAssignment(t *testing.T) {

	_, err := ParseAndCheckWithStorage(t,
		`
          fun test() {
              storage[Int] = "1"
              storage[Bool] = 1
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 2)

	assert.IsType(t, &sema.TypeMismatchError{}, errs[0])
	assert.IsType(t, &sema.TypeMismatchError{}, errs[1])
}

func TestCheckInvalidStorageIndexingAssignmentWithExpression(t *testing.T) {

	_, err := ParseAndCheckWithStorage(t,
		`
          fun test() {
              storage["1"] = "1"
          }
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidTypeIndexingError{}, errs[0])
}

func TestCheckInvalidStorageIndexingWithExpression(t *testing.T) {

	_, err := ParseAndCheckWithStorage(t,
		`
          let x = storage["1"]
        `,
	)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidTypeIndexingError{}, errs[0])
}

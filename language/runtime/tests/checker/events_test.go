package checker

import (
	"testing"

	"github.com/dapperlabs/flow-go/language/runtime/ast"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckEventDeclaration(t *testing.T) {
	t.Run("ValidEvent", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
			event Transfer(to: Int, from: Int)
		`)

		assert.Nil(t, err)
	})

	t.Run("InvalidEventNonPrimitiveType", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
			struct Token {
			  let ID: String
			
			  init(ID: String) {
				self.ID = ID
			  }
			}
			
			event Transfer(token: Token)
		`)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidEventParameterTypeError{}, errs[0])
	})

	t.Run("RedeclaredEvent", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
			event Transfer(to: Int, from: Int)
			event Transfer(to: Int)
		`)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.RedeclarationError{}, errs[0])
	})
}

func TestCheckEmitEvent(t *testing.T) {
	t.Run("ValidEvent", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
			event Transfer(to: Int, from: Int)
			
			fun test() {
			  emit Transfer(to: 1, from: 2)
			}
		`)

		assert.Nil(t, err)
	})

	t.Run("MissingEmitStatement", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
			event Transfer(to: Int, from: Int)
			
			fun test() {
			  Transfer(to: 1, from: 2)
			}
		`)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.InvalidEventUsageError{}, errs[0])
	})

	t.Run("EmitNonEvent", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
			fun notAnEvent(): Int { return 1 }			
			
			fun test() {
			  emit notAnEvent()
			}
		`)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.EmitNonEventError{}, errs[0])
	})

	t.Run("EmitNotDeclared", func(t *testing.T) {
		_, err := ParseAndCheck(t, `
			fun test() {
			  emit notAnEvent()
			}
		`)

		errs := ExpectCheckerErrors(t, err, 1)

		assert.IsType(t, &sema.NotDeclaredError{}, errs[0])
	})

	t.Run("EmitImported", func(t *testing.T) {
		checker, err := ParseAndCheck(t, `
			event Transfer(to: Int, from: Int)
		`)
		require.Nil(t, err)

		_, err = ParseAndCheckWithOptions(t, `
			import Transfer from "imported"

			pub fun test() {
				emit Transfer(to: 1, from: 2)
			}
			`,
			ParseAndCheckOptions{
				ImportResolver: func(location ast.Location) (program *ast.Program, e error) {
					return checker.Program, nil
				},
			},
		)

		errs := ExpectCheckerErrors(t, err, 2)
		assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
		assert.IsType(t, &sema.EmitImportedEventError{}, errs[1])
	})
}

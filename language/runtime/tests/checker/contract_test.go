package checker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/language/runtime/cmd"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	. "github.com/dapperlabs/flow-go/language/runtime/tests/utils"
)

func TestCheckInvalidContractAccountField(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {
          let account: Account

          init(account: Account) {
              self.account = account
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
}

func TestCheckInvalidContractAccountFunction(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {
          fun account() {}
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidDeclarationError{}, errs[0])
}

func TestCheckContractAccountFieldUse(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {

          init() {
              self.account.address
          }
      }
    `)

	require.NoError(t, err)
}

func TestCheckInvalidContractAccountFieldInitialization(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {

          init(account: Account) {
              self.account = account
          }
      }
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.AssignmentToConstantMemberError{}, errs[0])
}

func TestCheckInvalidContractAccountFieldAccess(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract Test {}

      let test = Test.account
    `)

	errs := ExpectCheckerErrors(t, err, 1)

	assert.IsType(t, &sema.InvalidAccessError{}, errs[0])
}

func TestCheckContractAccountFieldUseInitialized(t *testing.T) {

	code := `
      contract Test {
          let address: Address

          init() {
              // field 'account' can be used, as it is considered initialized
              self.address = self.account.address
          }

          fun test(): Address {
              return self.account.address
          }
      }

      let address1 = Test.address
      let address2 = Test.test()
    `
	_, err := ParseAndCheck(t, code)

	require.NoError(t, err)
}

func TestCheckInvalidContractMoveToFunction(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test() {
                              use(%s)
                          }
                      }

                      fun use(_ c: C) {}
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckInvalidContractMoveInVariableDeclaration(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test() {
                              let x = %s
                          }
                      }
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckInvalidContractMoveReturnFromFunction(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test(): C {
                              return %s
                          }
                      }
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckInvalidContractMoveIntoArrayLiteral(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test() {
                              let txs = [%s]
                          }
                      }
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckInvalidContractMoveIntoDictionaryLiteral(t *testing.T) {

	for _, name := range []string{"self", "C"} {

		t.Run(name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(
					`
                      contract C {

                          fun test() {
                              let txs = {"C": %s}
                          }
                      }
                    `,
					name,
				),
			)

			errs := ExpectCheckerErrors(t, err, 1)

			assert.IsType(t, &sema.InvalidMoveError{}, errs[0])
		})
	}
}

func TestCheckContractNestedDeclarationOrderOutsideInside(t *testing.T) {

	for _, isInterface := range []bool{true, false} {

		interfaceKeyword := ""
		if isInterface {
			interfaceKeyword = "interface"
		}

		body := ""
		if !isInterface {
			body = "{}"
		}

		extraFunction := ""
		if !isInterface {
			extraFunction = `
		      fun callGoNew() {
                  let r <- create R()
                  r.go()
                  destroy r
              }
            `
		}

		t.Run(interfaceKeyword, func(t *testing.T) {

			code := fmt.Sprintf(
				`
                  contract C {

                      fun callGoExisting(r: @R) {
                          r.go()
                          destroy r
                      }

                      %[1]s

                      resource %[2]s R {
                          fun go() %[3]s
                      }
                  }
                `,
				extraFunction,
				interfaceKeyword,
				body,
			)
			_, err := ParseAndCheck(t, code)

			if !assert.NoError(t, err) {
				cmd.PrettyPrintError(err, "", map[string]string{"": code})
			}
		})
	}
}

func TestCheckContractNestedDeclarationOrderInsideOutside(t *testing.T) {

	_, err := ParseAndCheck(t, `
      contract C {

          fun go() {}

          resource R {
              fun callGo() {
                  C.go()
              }
          }
      }
    `)

	require.NoError(t, err)
}

// TestCheckContractNestedDeclarationsComplex tests
// - Using inner types in functions outside (both type in parameter and constructor)
// - Using outer functions in inner types' functions
// - Mutually using sibling types
//
func TestCheckContractNestedDeclarationsComplex(t *testing.T) {

	interfacePossibilities := []bool{true, false}

	compositeKinds := []common.CompositeKind{
		common.CompositeKindStructure,
		common.CompositeKindResource,
	}

	for _, contractIsInterface := range interfacePossibilities {
		for _, firstKind := range compositeKinds {
			for _, firstIsInterface := range interfacePossibilities {
				for _, secondKind := range compositeKinds {
					for _, secondIsInterface := range interfacePossibilities {

						contractInterfaceKeyword := ""
						if contractIsInterface {
							contractInterfaceKeyword = "interface"
						}

						firstInterfaceKeyword := ""
						if firstIsInterface {
							firstInterfaceKeyword = "interface"
						}

						secondInterfaceKeyword := ""
						if secondIsInterface {
							secondInterfaceKeyword = "interface"
						}

						testName := fmt.Sprintf(
							"contract_%s/%s_%s/%s_%s",
							contractInterfaceKeyword,
							firstKind.Keyword(),
							firstInterfaceKeyword,
							secondKind.Keyword(),
							secondInterfaceKeyword,
						)

						bodyUsingFirstOutside := ""
						if !contractIsInterface {
							if secondIsInterface {
								bodyUsingFirstOutside = fmt.Sprintf(
									"{ %s a }",
									firstKind.DestructionKeyword(),
								)
							} else {
								bodyUsingFirstOutside = fmt.Sprintf(
									"{ a.localB(%[1]s %[2]s B()); %[3]s a }",
									secondKind.MoveOperator(),
									secondKind.ConstructionKeyword(),
									firstKind.DestructionKeyword(),
								)
							}
						}

						bodyUsingSecondOutside := ""
						if !contractIsInterface {
							if firstIsInterface {
								bodyUsingSecondOutside = fmt.Sprintf(
									"{ %s b }",
									secondKind.DestructionKeyword(),
								)
							} else {
								bodyUsingSecondOutside = fmt.Sprintf(
									"{ b.localA(%[1]s %[2]s A()); %[3]s b }",
									firstKind.MoveOperator(),
									firstKind.ConstructionKeyword(),
									secondKind.DestructionKeyword(),
								)
							}
						}

						bodyUsingFirstInsideFirst := ""
						bodyUsingSecondInsideFirst := ""
						bodyUsingFirstInsideSecond := ""
						bodyUsingSecondInsideSecond := ""

						if !contractIsInterface && !firstIsInterface {
							bodyUsingFirstInsideFirst = fmt.Sprintf(
								"{ C.localBeforeA(%s a) }",
								firstKind.MoveOperator(),
							)
							bodyUsingSecondInsideFirst = fmt.Sprintf(
								"{ C.localBeforeB(%s b)  }",
								secondKind.MoveOperator(),
							)
						}

						if !contractIsInterface && !secondIsInterface {
							bodyUsingFirstInsideSecond = fmt.Sprintf(
								"{ C.qualifiedAfterA(%s a) }",
								firstKind.MoveOperator(),
							)
							bodyUsingSecondInsideSecond = fmt.Sprintf(
								"{ C.qualifiedAfterB(%s b)  }",
								secondKind.MoveOperator(),
							)
						}

						t.Run(testName, func(t *testing.T) {

							code := fmt.Sprintf(
								`
                                  contract %[1]s C {

                                      fun qualifiedBeforeA(_ a: %[4]sC.A) %[12]s
                                      fun localBeforeA(_ a: %[4]sA) %[12]s

                                      fun qualifiedBeforeB(_ b: %[7]sC.B) %[13]s
                                      fun localBeforeB(_ b: %[7]sB) %[13]s

                                      %[2]s %[3]s A {
                                          fun qualifiedB(_ b: %[7]sC.B) %[9]s
                                          fun localB(_ b: %[7]sB) %[9]s

                                          fun qualifiedA(_ a: %[4]sC.A) %[8]s
                                          fun localA(_ a: %[4]sA) %[8]s
                                      }

                                      %[5]s %[6]s B {
                                          fun qualifiedA(_ a: %[4]sC.A) %[10]s
                                          fun localA(_ a: %[4]sA) %[10]s

                                          fun qualifiedB(_ b: %[7]sC.B) %[11]s
                                          fun localB(_ b: %[7]sB) %[11]s
                                      }

                                      fun qualifiedAfterA(_ a: %[4]sC.A) %[12]s
                                      fun localAfterA(_ a: %[4]sA) %[12]s

                                      fun qualifiedAfterB(_ b: %[7]sC.B) %[13]s
                                      fun localAfterB(_ b: %[7]sB) %[13]s
                                  }
                                `,
								contractInterfaceKeyword,
								firstKind.Keyword(),
								firstInterfaceKeyword,
								firstKind.Annotation(),
								secondKind.Keyword(),
								secondInterfaceKeyword,
								secondKind.Annotation(),
								bodyUsingFirstInsideFirst,
								bodyUsingSecondInsideFirst,
								bodyUsingFirstInsideSecond,
								bodyUsingSecondInsideSecond,
								bodyUsingFirstOutside,
								bodyUsingSecondOutside,
							)
							_, err := ParseAndCheck(t, code)

							if !assert.NoError(t, err) {
								cmd.PrettyPrintError(err, "", map[string]string{"": code})
							}
						})
					}
				}
			}
		}
	}
}

func TestCheckInvalidContractNestedTypeShadowing(t *testing.T) {

	type test struct {
		name        string
		code        string
		isInterface bool
	}

	tests := []test{
		{name: "event", code: `event Test()`, isInterface: false},
	}

	for _, kind := range common.CompositeKindsWithBody {

		// Contracts can not be nested
		if kind == common.CompositeKindContract {
			continue
		}

		for _, isInterface := range []bool{true, false} {
			keywords := kind.Keyword()

			if isInterface {
				keywords += " interface"
			}

			code := fmt.Sprintf(`%s Test {}`, keywords)

			tests = append(tests, test{
				name:        keywords,
				code:        code,
				isInterface: isInterface,
			})
		}
	}

	for _, test := range tests {

		t.Run(test.name, func(t *testing.T) {

			_, err := ParseAndCheck(t,
				fmt.Sprintf(`
                      contract Test {
                          %s
                      }
                    `,
					test.code,
				),
			)

			// If the nested element is an interface, there will only be an error
			// for the redeclared type.
			//
			// If the nested element is a concrete type, there will also be an error
			// for the redeclared value (constructor).

			expectedErrors := 1
			if !test.isInterface {
				expectedErrors += 1
			}

			errs := ExpectCheckerErrors(t, err, expectedErrors)

			for i := 0; i < expectedErrors; i++ {
				assert.IsType(t, &sema.RedeclarationError{}, errs[i])
			}
		})
	}
}

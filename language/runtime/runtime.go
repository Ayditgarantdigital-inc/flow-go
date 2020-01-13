package runtime

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/language/runtime/ast"
	"github.com/dapperlabs/flow-go/language/runtime/common"
	runtimeErrors "github.com/dapperlabs/flow-go/language/runtime/errors"
	"github.com/dapperlabs/flow-go/language/runtime/interpreter"
	"github.com/dapperlabs/flow-go/language/runtime/parser"
	"github.com/dapperlabs/flow-go/language/runtime/sema"
	"github.com/dapperlabs/flow-go/language/runtime/stdlib"
	"github.com/dapperlabs/flow-go/language/runtime/trampoline"
	"github.com/dapperlabs/flow-go/model/flow"
)

type Interface interface {
	// ResolveImport resolves an import of a program.
	ResolveImport(Location) ([]byte, error)
	// GetValue gets a value for the given key in the storage, controlled and owned by the given accounts.
	GetValue(owner, controller, key []byte) (value []byte, err error)
	// SetValue sets a value for the given key in the storage, controlled and owned by the given accounts.
	SetValue(owner, controller, key, value []byte) (err error)
	// CreateAccount creates a new account with the given public keys and code.
	CreateAccount(publicKeys [][]byte) (address Address, err error)
	// AddAccountKey appends a key to an account.
	AddAccountKey(address Address, publicKey []byte) error
	// RemoveAccountKey removes a key from an account by index.
	RemoveAccountKey(address Address, index int) (publicKey []byte, err error)
	// CheckCode checks the validity of the code.
	CheckCode(address Address, code []byte) (err error)
	// UpdateAccountCode updates the code associated with an account.
	UpdateAccountCode(address Address, code []byte, checkPermission bool) (err error)
	// GetSigningAccounts returns the signing accounts.
	GetSigningAccounts() []Address
	// Log logs a string.
	Log(string)
	// EmitEvent is called when an event is emitted by the runtime.
	EmitEvent(Event)
}

// Runtime is a runtime capable of executing the Flow programming language.
type Runtime interface {
	// ExecuteScript executes the given script.
	//
	// This function returns an error if the program has errors (e.g syntax errors, type errors),
	// or if the execution fails.
	ExecuteScript(script []byte, runtimeInterface Interface, location Location) (Value, error)

	// ExecuteTransaction executes the given transaction.
	//
	// This function returns an error if the program has errors (e.g syntax errors, type errors),
	// or if the execution fails.
	ExecuteTransaction(script []byte, runtimeInterface Interface, location Location) error

	// ParseAndCheckProgram parses and checks the given code without executing the program.
	//
	// This function returns an error if the program contains any syntax or semantic errors.
	ParseAndCheckProgram(code []byte, runtimeInterface Interface, location Location) error
}

var typeDeclarations = stdlib.BuiltinTypes.ToTypeDeclarations()

type ImportResolver = func(location Location) (program *ast.Program, e error)

var validTopLevelDeclarations = []common.DeclarationKind{
	common.DeclarationKindImport,
	common.DeclarationKindContract,
	common.DeclarationKindContractInterface,
}

const contractKey = "contract"

// interpreterRuntime is a interpreter-based version of the Flow runtime.
type interpreterRuntime struct{}

// NewInterpreterRuntime returns a interpreter-based version of the Flow runtime.
func NewInterpreterRuntime() Runtime {
	return &interpreterRuntime{}
}

func (r *interpreterRuntime) ExecuteScript(script []byte, runtimeInterface Interface, location Location) (Value, error) {
	runtimeStorage := newInterpreterRuntimeStorage(runtimeInterface)

	functions := r.standardLibraryFunctions(runtimeInterface, runtimeStorage)

	checker, err := r.parseAndCheckProgram(script, runtimeInterface, location, functions, nil)
	if err != nil {
		return nil, newError(err)
	}

	_, ok := checker.GlobalValues["main"]
	if !ok {
		// TODO: error because no main?
		return nil, nil
	}

	value, err := r.interpret(
		runtimeInterface,
		runtimeStorage,
		checker,
		functions,
		nil,
		func(inter *interpreter.Interpreter) (interpreter.Value, error) {
			return inter.Invoke("main")
		},
	)
	if err != nil {
		return nil, newError(err)
	}

	// Write back all stored values, which were actually just cached, back into storage.

	// Even though this function is `ExecuteScript`, that doesn't imply the changes
	// to storage will be actually persisted

	runtimeStorage.writeCached()

	return value, nil
}

func (r *interpreterRuntime) interpret(
	runtimeInterface Interface,
	runtimeStorage *interpreterRuntimeStorage,
	checker *sema.Checker,
	functions stdlib.StandardLibraryFunctions,
	options []interpreter.Option,
	f func(inter *interpreter.Interpreter) (interpreter.Value, error),
) (
	interpreter.Value,
	error,
) {
	inter, err := r.newInterpreter(checker, functions, runtimeInterface, runtimeStorage, options)
	if err != nil {
		return nil, err
	}

	if err := inter.Interpret(); err != nil {
		return nil, err
	}

	if f != nil {
		value, err := f(inter)
		if err != nil {
			return nil, err
		}
		return value, nil
	}

	return nil, nil
}

func (r *interpreterRuntime) ExecuteTransaction(
	script []byte,
	runtimeInterface Interface,
	location Location,
) error {
	runtimeStorage := newInterpreterRuntimeStorage(runtimeInterface)

	functions := r.standardLibraryFunctions(runtimeInterface, runtimeStorage)

	checker, err := r.parseAndCheckProgram(script, runtimeInterface, location, functions, nil)
	if err != nil {
		return newError(err)
	}

	transactions := checker.TransactionTypes
	transactionCount := len(transactions)
	if transactionCount != 1 {
		return newError(InvalidTransactionCountError{Count: transactionCount})
	}

	transactionType := transactions[0]
	transactionFunctionType := transactionType.EntryPointFunctionType()

	signingAccountAddresses := runtimeInterface.GetSigningAccounts()

	// check parameter count

	signingAccountsCount := len(signingAccountAddresses)
	transactionFunctionParameterCount := len(transactionFunctionType.Parameters)
	if signingAccountsCount != transactionFunctionParameterCount {
		return newError(InvalidTransactionParameterCountError{
			Expected: transactionFunctionParameterCount,
			Actual:   signingAccountsCount,
		})
	}

	// check parameter types

	for _, parameter := range transactionFunctionType.Parameters {
		parameterType := parameter.TypeAnnotation.Type

		if !parameterType.Equal(&sema.AccountType{}) {
			return newError(InvalidTransactionParameterTypeError{
				Actual: parameterType,
			})
		}
	}

	signingAccounts := make([]interface{}, signingAccountsCount)

	for i, address := range signingAccountAddresses {
		signingAccounts[i] = interpreter.NewAccountValue(address)
	}

	_, err = r.interpret(
		runtimeInterface,
		runtimeStorage,
		checker,
		functions,
		nil,
		func(inter *interpreter.Interpreter) (interpreter.Value, error) {
			err := inter.InvokeTransaction(0, signingAccounts...)
			return nil, err
		},
	)
	if err != nil {
		return newError(err)
	}

	// Write back all stored values, which were actually just cached, back into storage
	runtimeStorage.writeCached()

	return nil
}

func (r *interpreterRuntime) ParseAndCheckProgram(script []byte, runtimeInterface Interface, location Location) error {
	runtimeStorage := newInterpreterRuntimeStorage(runtimeInterface)
	functions := r.standardLibraryFunctions(runtimeInterface, runtimeStorage)

	_, err := r.parseAndCheckProgram(script, runtimeInterface, location, functions, nil)
	if err != nil {
		return newError(err)
	}

	return nil
}

func (r *interpreterRuntime) parseAndCheckProgram(
	code []byte,
	runtimeInterface Interface,
	location Location,
	functions stdlib.StandardLibraryFunctions,
	options []sema.Option,
) (*sema.Checker, error) {
	program, err := r.parse(code)
	if err != nil {
		return nil, err
	}

	importResolver := r.importResolver(runtimeInterface)
	err = program.ResolveImports(importResolver)
	if err != nil {
		return nil, err
	}

	valueDeclarations := functions.ToValueDeclarations()

	checker, err := sema.NewChecker(
		program,
		location,
		append(
			[]sema.Option{
				sema.WithPredeclaredValues(valueDeclarations),
				sema.WithPredeclaredTypes(typeDeclarations),
			},
			options...,
		)...,
	)
	if err != nil {
		return nil, err
	}

	if err := checker.Check(); err != nil {
		return nil, err
	}

	return checker, nil
}

func (r *interpreterRuntime) newInterpreter(
	checker *sema.Checker,
	functions stdlib.StandardLibraryFunctions,
	runtimeInterface Interface,
	runtimeStorage *interpreterRuntimeStorage,
	options []interpreter.Option,
) (*interpreter.Interpreter, error) {

	importResolver := r.importResolver(runtimeInterface)

	defaultOptions := []interpreter.Option{
		interpreter.WithPredefinedValues(functions.ToValues()),
		interpreter.WithOnEventEmittedHandler(
			func(
				inter *interpreter.Interpreter,
				eventValue *interpreter.CompositeValue,
				eventType *sema.CompositeType,
			) {
				r.emitEvent(inter, runtimeInterface, eventValue, eventType)
			},
		),
		interpreter.WithStorageReadHandler(
			func(_ *interpreter.Interpreter, storageIdentifier string, key string) interpreter.OptionalValue {
				return runtimeStorage.readValue(storageIdentifier, key)
			},
		),
		interpreter.WithStorageWriteHandler(
			func(_ *interpreter.Interpreter, storageIdentifier string, key string, value interpreter.OptionalValue) {
				runtimeStorage.writeValue(storageIdentifier, key, value)
			},
		),
		interpreter.WithStorageKeyHandler(
			func(_ *interpreter.Interpreter, _ string, indexingType sema.Type) string {
				return string(indexingType.ID())
			},
		),
		interpreter.WithInjectedCompositeFieldsHandler(
			func(
				_ *interpreter.Interpreter,
				location Location,
				_ sema.TypeID,
				compositeKind common.CompositeKind,
			) map[string]interpreter.Value {

				switch compositeKind {
				case common.CompositeKindContract:
					var address []byte

					switch location := location.(type) {
					case AddressLocation:
						address = location
					default:
						panic(runtimeErrors.NewUnreachableError())
					}

					addressLocation := interpreter.NewAddressValueFromBytes(address)

					return map[string]interpreter.Value{
						"account": interpreter.NewAccountValue(addressLocation),
					}
				}

				return nil
			},
		),
		interpreter.WithContractValueHandler(
			func(
				inter *interpreter.Interpreter,
				compositeType *sema.CompositeType,
				_ interpreter.FunctionValue,
			) *interpreter.CompositeValue {
				// Load the contract from storage
				return r.loadContract(compositeType, runtimeStorage)
			},
		),
		interpreter.WithImportProgramHandler(
			func(inter *interpreter.Interpreter, location ast.Location) *ast.Program {
				program, err := importResolver(location)
				if err != nil {
					panic(err)
				}

				err = program.ResolveImports(importResolver)
				if err != nil {
					panic(err)
				}

				return program
			},
		),
	}

	return interpreter.NewInterpreter(
		checker,
		append(defaultOptions, options...)...,
	)
}

func (r *interpreterRuntime) standardLibraryFunctions(
	runtimeInterface Interface,
	runtimeStorage *interpreterRuntimeStorage,
) stdlib.StandardLibraryFunctions {
	return append(
		stdlib.FlowBuiltInFunctions(stdlib.FlowBuiltinImpls{
			CreateAccount:     r.newCreateAccountFunction(runtimeInterface, runtimeStorage),
			AddAccountKey:     r.newAddAccountKeyFunction(runtimeInterface),
			RemoveAccountKey:  r.newRemoveAccountKeyFunction(runtimeInterface),
			UpdateAccountCode: r.newUpdateAccountCodeFunction(runtimeInterface, runtimeStorage),
			GetAccount:        r.newGetAccountFunction(runtimeInterface),
			Log:               r.newLogFunction(runtimeInterface),
		}),
		stdlib.BuiltinFunctions...,
	)
}

func (r *interpreterRuntime) importResolver(runtimeInterface Interface) ImportResolver {
	return func(location Location) (program *ast.Program, e error) {
		script, err := runtimeInterface.ResolveImport(location)
		if err != nil {
			return nil, err
		}
		return r.parse(script)
	}
}

func (r *interpreterRuntime) parse(script []byte) (program *ast.Program, err error) {
	program, _, err = parser.ParseProgram(string(script))
	return
}

// emitEvent converts an event value to native Go types and emits it to the runtime interface.
func (r *interpreterRuntime) emitEvent(
	_ *interpreter.Interpreter,
	runtimeInterface Interface,
	event *interpreter.CompositeValue,
	eventType *sema.CompositeType,
) {
	fields := make([]Value, len(eventType.ConstructorParameters))

	for i, parameter := range eventType.ConstructorParameters {
		fields[i] = event.Fields[parameter.Identifier]
	}

	eventValue := Event{
		Type:   eventType,
		Fields: fields,
	}

	runtimeInterface.EmitEvent(eventValue)
}

func (r *interpreterRuntime) emitAccountEvent(
	eventType *sema.CompositeType,
	runtimeInterface Interface,
	eventFields []interpreter.Value,
) {
	eventValue := Event{
		Type:   eventType,
		Fields: eventFields,
	}

	runtimeInterface.EmitEvent(eventValue)
}

func (r *interpreterRuntime) newCreateAccountFunction(
	runtimeInterface Interface,
	runtimeStorage *interpreterRuntimeStorage,
) interpreter.HostFunction {
	return func(invocation interpreter.Invocation) trampoline.Trampoline {
		const requiredArgumentCount = 2

		pkArray := invocation.Arguments[0].(*interpreter.ArrayValue)
		pkValues := pkArray.Values
		publicKeys := make([][]byte, len(pkValues))

		for i, pkVal := range pkValues {
			publicKey, err := toBytes(pkVal)
			if err != nil {
				panic(fmt.Sprintf("createAccount requires the first parameter to be an array of keys ([[Int]])"))
			}
			publicKeys[i] = publicKey
		}

		code, err := toBytes(invocation.Arguments[1])
		if err != nil {
			panic(fmt.Sprintf("createAccount requires the second parameter to be an array of bytes ([Int])"))
		}

		accountAddress, err := runtimeInterface.CreateAccount(publicKeys)
		if err != nil {
			panic(err)
		}

		constructorArguments := invocation.Arguments[requiredArgumentCount:]
		constructorArgumentTypes := invocation.ArgumentTypes[requiredArgumentCount:]

		r.updateAccountCode(
			runtimeInterface,
			runtimeStorage,
			code,
			accountAddress,
			constructorArguments,
			constructorArgumentTypes,
			false,
			invocation.Location.Position,
		)

		r.emitAccountEvent(
			stdlib.AccountCreatedEventType,
			runtimeInterface,
			[]Value{accountAddress},
		)

		return trampoline.Done{Result: accountAddress}
	}
}

func (r *interpreterRuntime) newAddAccountKeyFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(invocation interpreter.Invocation) trampoline.Trampoline {
		accountAddress := invocation.Arguments[0].(interpreter.AddressValue)
		publicKeyValue := invocation.Arguments[1].(*interpreter.ArrayValue)

		publicKey, err := toBytes(publicKeyValue)
		if err != nil {
			panic(fmt.Sprintf("addAccountKey requires the second parameter to be an array"))
		}

		err = runtimeInterface.AddAccountKey(accountAddress, publicKey)
		if err != nil {
			panic(err)
		}

		r.emitAccountEvent(
			stdlib.AccountKeyAddedEventType,
			runtimeInterface,
			[]Value{accountAddress, publicKeyValue},
		)

		result := interpreter.VoidValue{}
		return trampoline.Done{Result: result}
	}
}

func (r *interpreterRuntime) newRemoveAccountKeyFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(invocation interpreter.Invocation) trampoline.Trampoline {
		accountAddress := invocation.Arguments[0].(interpreter.AddressValue)
		index := invocation.Arguments[1].(interpreter.IntValue)

		publicKey, err := runtimeInterface.RemoveAccountKey(accountAddress, index.IntValue())
		if err != nil {
			panic(err)
		}

		publicKeyValue := fromBytes(publicKey)

		r.emitAccountEvent(
			stdlib.AccountKeyRemovedEventType,
			runtimeInterface,
			[]Value{accountAddress, publicKeyValue},
		)

		result := interpreter.VoidValue{}
		return trampoline.Done{Result: result}
	}
}

func (r *interpreterRuntime) newUpdateAccountCodeFunction(
	runtimeInterface Interface,
	runtimeStorage *interpreterRuntimeStorage,
) interpreter.HostFunction {
	return func(invocation interpreter.Invocation) trampoline.Trampoline {
		const requiredArgumentCount = 2

		accountAddress := invocation.Arguments[0].(interpreter.AddressValue)

		code, err := toBytes(invocation.Arguments[1])
		if err != nil {
			panic(fmt.Sprintf("updateAccountCode requires the second parameter to be an array of bytes ([Int])"))
		}

		constructorArguments := invocation.Arguments[requiredArgumentCount:]
		constructorArgumentTypes := invocation.ArgumentTypes[requiredArgumentCount:]

		r.updateAccountCode(
			runtimeInterface,
			runtimeStorage,
			code,
			accountAddress,
			constructorArguments,
			constructorArgumentTypes,
			true,
			invocation.Location.Position,
		)

		codeValue := fromBytes(code)

		r.emitAccountEvent(
			stdlib.AccountCodeUpdatedEventType,
			runtimeInterface,
			[]Value{accountAddress, codeValue},
		)

		result := interpreter.VoidValue{}
		return trampoline.Done{Result: result}
	}
}

func (r *interpreterRuntime) updateAccountCode(
	runtimeInterface Interface,
	runtimeStorage *interpreterRuntimeStorage,
	code []byte,
	accountAddress Address,
	constructorArguments []interpreter.Value,
	constructorArgumentTypes []sema.Type,
	checkPermission bool,
	invocationPosition ast.Position,
) {
	location := AddressLocation(accountAddress[:])

	functions := r.standardLibraryFunctions(runtimeInterface, runtimeStorage)
	checker, err := r.parseAndCheckProgram(
		code,
		runtimeInterface,
		location,
		functions,
		[]sema.Option{
			sema.WithValidTopLevelDeclarations(validTopLevelDeclarations),
		},
	)
	if err != nil {
		panic(err)
	}

	var contractTypes []*sema.CompositeType

	for _, variable := range checker.GlobalTypes {
		if variable.DeclarationKind == common.DeclarationKindContract {
			contractType := variable.Type.(*sema.CompositeType)
			contractTypes = append(contractTypes, contractType)
		}
	}

	if len(contractTypes) > 1 {
		panic(fmt.Sprintf("code declares more than one contract"))
	}

	// If the code declares a contract, instantiate it and store it

	var contractValue interpreter.OptionalValue = interpreter.NilValue{}

	if len(contractTypes) > 0 {
		contractType := contractTypes[0]

		contract, err := r.instantiateContract(
			contractType,
			constructorArguments,
			constructorArgumentTypes,
			runtimeInterface,
			runtimeStorage,
			checker,
			functions,
			invocationPosition,
		)

		if err != nil {
			panic(err)
		}

		contractValue = interpreter.NewSomeValueOwningNonCopying(contract)
	}

	contractValue.SetOwner(accountAddress.Hex())

	// NOTE: only update account code if contract instantiation succeeded

	err = runtimeInterface.UpdateAccountCode(accountAddress, code, checkPermission)
	if err != nil {
		panic(err)
	}

	r.writeContract(runtimeStorage, accountAddress, contractValue)
}

func (r *interpreterRuntime) writeContract(
	runtimeStorage *interpreterRuntimeStorage,
	accountAddress Address,
	contractValue interpreter.OptionalValue,
) {
	addressHex := flow.BytesToAddress(accountAddress[:]).Hex()
	runtimeStorage.writeValue(
		addressHex,
		contractKey,
		contractValue,
	)
}

func (r *interpreterRuntime) loadContract(
	compositeType *sema.CompositeType,
	runtimeStorage *interpreterRuntimeStorage,
) *interpreter.CompositeValue {
	addressHex := compositeType.Location.(AddressLocation).ToAddress().Hex()
	storedValue := runtimeStorage.readValue(
		addressHex,
		contractKey,
	)
	switch typedValue := storedValue.(type) {
	case *interpreter.SomeValue:
		return typedValue.Value.(*interpreter.CompositeValue)
	case interpreter.NilValue:
		panic("failed to load contract")
	default:
		panic(runtimeErrors.NewUnreachableError())
	}
}

func (r *interpreterRuntime) instantiateContract(
	contractType *sema.CompositeType,
	constructorArguments []interpreter.Value,
	argumentTypes []sema.Type,
	runtimeInterface Interface,
	runtimeStorage *interpreterRuntimeStorage,
	checker *sema.Checker,
	functions stdlib.StandardLibraryFunctions,
	invocationPos ast.Position,
) (
	interpreter.Value,
	error,
) {
	parameterTypes := make([]sema.Type, len(contractType.ConstructorParameters))

	for i, constructorParameter := range contractType.ConstructorParameters {
		parameterTypes[i] = constructorParameter.TypeAnnotation.Type
	}

	// Check argument count

	argumentCount := len(argumentTypes)
	parameterCount := len(parameterTypes)

	if argumentCount != parameterCount {
		return nil, fmt.Errorf("invalid argument count: expected %d, got %d", parameterCount, argumentCount)
	}

	// Check arguments match parameter

	for i := 0; i < argumentCount; i++ {
		argumentType := argumentTypes[i]
		parameterTye := parameterTypes[i]
		if !sema.IsSubType(argumentType, parameterTye) {
			return nil, fmt.Errorf(
				"invalid argument %d: expected type `%s`, got `%s`",
				i,
				parameterTye,
				argumentType,
			)
		}
	}

	// Use a custom contract value handler that detects if the requested contract value
	// is for the contract declaration that is being deployed.
	//
	// If the contract is the deployed contract, instantiate it using
	// the provided constructor and given arguments.
	//
	// If the contract is not the deployed contract, load it from storage.

	var contract *interpreter.CompositeValue

	interpreterOptions := []interpreter.Option{
		interpreter.WithContractValueHandler(
			func(
				inter *interpreter.Interpreter,
				compositeType *sema.CompositeType,
				constructor interpreter.FunctionValue,
			) *interpreter.CompositeValue {

				// If the contract is the deployed contract, instantiate it using
				// the provided constructor and given arguments

				if ast.LocationsMatch(compositeType.Location, contractType.Location) &&
					compositeType.Identifier == contractType.Identifier {

					value, err := inter.InvokeFunctionValue(constructor,
						constructorArguments,
						argumentTypes,
						parameterTypes,
						invocationPos,
					)
					if err != nil {
						panic(err)
					}

					contract = value.(*interpreter.CompositeValue)

					return contract
				} else {
					// The contract is not the deployed contract, load it from storage

					return r.loadContract(compositeType, runtimeStorage)
				}
			},
		),
	}

	_, err := r.interpret(
		runtimeInterface,
		runtimeStorage,
		checker,
		functions,
		interpreterOptions,
		nil,
	)

	if err != nil {
		return nil, err
	}

	return contract, err
}

func (r *interpreterRuntime) newGetAccountFunction(_ Interface) interpreter.HostFunction {
	return func(invocation interpreter.Invocation) trampoline.Trampoline {
		accountAddress := invocation.Arguments[0].(interpreter.AddressValue)
		publicAccount := interpreter.NewPublicAccountValue(accountAddress)
		return trampoline.Done{Result: publicAccount}
	}
}

func (r *interpreterRuntime) newLogFunction(runtimeInterface Interface) interpreter.HostFunction {
	return func(invocation interpreter.Invocation) trampoline.Trampoline {
		runtimeInterface.Log(fmt.Sprint(invocation.Arguments[0]))
		result := interpreter.VoidValue{}
		return trampoline.Done{Result: result}
	}
}

func toBytes(value interpreter.Value) ([]byte, error) {
	_, isNil := value.(interpreter.NilValue)
	if isNil {
		return nil, nil
	}

	someValue, ok := value.(*interpreter.SomeValue)
	if ok {
		value = someValue.Value
	}

	array, ok := value.(*interpreter.ArrayValue)
	if !ok {
		return nil, errors.New("value is not an array")
	}

	result := make([]byte, len(array.Values))
	for i, arrayValue := range array.Values {
		intValue, ok := arrayValue.(interpreter.IntValue)
		if !ok {
			return nil, errors.New("array value is not an Int")
		}

		j := intValue.IntValue()

		if j < 0 || j > 255 {
			return nil, errors.New("array value is not in byte range (0-255)")
		}

		result[i] = byte(j)
	}

	return result, nil
}

func fromBytes(buf []byte) *interpreter.ArrayValue {
	values := make([]interpreter.Value, len(buf))
	for i, b := range buf {
		values[i] = interpreter.NewIntValue(int64(b))
	}

	return &interpreter.ArrayValue{
		Values: values,
	}
}

package interpreter

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/norm"

	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/errors"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/sema"
	"github.com/dapperlabs/bamboo-node/pkg/language/runtime/trampoline"
)

type Value interface {
	isValue()
	Copy() Value
}

type ExportableValue interface {
	ToGoValue() interface{}
}

// VoidValue

type VoidValue struct{}

func (VoidValue) isValue() {}

func (v VoidValue) Copy() Value {
	return v
}

func (v VoidValue) ToGoValue() interface{} {
	return nil
}

func (v VoidValue) String() string {
	return "()"
}

// BoolValue

type BoolValue bool

func (BoolValue) isValue() {}

func (v BoolValue) Copy() Value {
	return v
}

func (v BoolValue) ToGoValue() interface{} {
	return bool(v)
}

func (v BoolValue) Negate() BoolValue {
	return !v
}

// StringValue

type StringValue string

func (StringValue) isValue() {}

func (v StringValue) Copy() Value {
	return v
}

func (v StringValue) ToGoValue() interface{} {
	return string(v)
}

func (v StringValue) String() string {
	// TODO: quote like in string literal
	return strconv.Quote(string(v))
}

func (v StringValue) Equal(other StringValue) BoolValue {
	return norm.NFC.String(string(v)) == norm.NFC.String(string(other))
}

func (v StringValue) GetMember(interpreter *Interpreter, name string) Value {
	switch name {
	case "length":
		count := uniseg.GraphemeClusterCount(string(v))
		return IntValue{Int: big.NewInt(int64(count))}
	default:
		panic(&errors.UnreachableError{})
	}
}

func (v StringValue) SetMember(interpreter *Interpreter, name string, value Value) {
	panic(&errors.UnreachableError{})
}

// IndexableValue

type IndexableValue interface {
	isIndexableValue()
	Get(key Value) Value
	Set(key Value, value Value)
}

// ArrayValue

type ArrayValue struct {
	Values *[]Value
}

func (ArrayValue) isValue() {}

func (v ArrayValue) Copy() Value {
	// TODO: optimize, use copy-on-write
	copies := make([]Value, len(*v.Values))
	for i, value := range *v.Values {
		copies[i] = value.Copy()
	}
	return ArrayValue{Values: &copies}
}

func (v ArrayValue) ToGoValue() interface{} {
	values := make([]interface{}, len(*v.Values))

	for i, value := range *v.Values {
		values[i] = value.(ExportableValue).ToGoValue()
	}

	return values
}

func (v ArrayValue) isIndexableValue() {}

func (v ArrayValue) Get(key Value) Value {
	return (*v.Values)[key.(IntegerValue).IntValue()]
}

func (v ArrayValue) Set(key Value, value Value) {
	(*v.Values)[key.(IntegerValue).IntValue()] = value
}

func (v ArrayValue) String() string {
	var builder strings.Builder
	builder.WriteString("[")
	for i, value := range *v.Values {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprint(value))
	}
	builder.WriteString("]")
	return builder.String()
}

func (v ArrayValue) GetMember(interpreter *Interpreter, name string) Value {
	switch name {
	case "length":
		return IntValue{Int: big.NewInt(int64(len(*v.Values)))}
	case "append":
		return NewHostFunctionValue(
			func(arguments []Value, location Location) trampoline.Trampoline {
				*v.Values = append(*v.Values, arguments[0])
				return trampoline.Done{Result: VoidValue{}}
			},
		)
	default:
		panic(&errors.UnreachableError{})
	}
}

func (v ArrayValue) SetMember(interpreter *Interpreter, name string, value Value) {
	panic(&errors.UnreachableError{})
}

// IntegerValue

type IntegerValue interface {
	Value
	IntValue() int
	Negate() IntegerValue
	Plus(other IntegerValue) IntegerValue
	Minus(other IntegerValue) IntegerValue
	Mod(other IntegerValue) IntegerValue
	Mul(other IntegerValue) IntegerValue
	Div(other IntegerValue) IntegerValue
	Less(other IntegerValue) BoolValue
	LessEqual(other IntegerValue) BoolValue
	Greater(other IntegerValue) BoolValue
	GreaterEqual(other IntegerValue) BoolValue
	Equal(other IntegerValue) BoolValue
}

// IntValue

type IntValue struct {
	*big.Int
}

func (v IntValue) isValue() {}

func (v IntValue) Copy() Value {
	return IntValue{big.NewInt(0).Set(v.Int)}
}

func (v IntValue) ToGoValue() interface{} {
	return big.NewInt(0).Set(v.Int)
}

func (v IntValue) IntValue() int {
	// TODO: handle overflow
	return int(v.Int64())
}

func (v IntValue) KeyString() string {
	return v.Int.String()
}

func (v IntValue) Negate() IntegerValue {
	return IntValue{big.NewInt(0).Neg(v.Int)}
}

func (v IntValue) Plus(other IntegerValue) IntegerValue {
	newValue := big.NewInt(0).Add(v.Int, other.(IntValue).Int)
	return IntValue{newValue}
}

func (v IntValue) Minus(other IntegerValue) IntegerValue {
	newValue := big.NewInt(0).Sub(v.Int, other.(IntValue).Int)
	return IntValue{newValue}
}

func (v IntValue) Mod(other IntegerValue) IntegerValue {
	newValue := big.NewInt(0).Mod(v.Int, other.(IntValue).Int)
	return IntValue{newValue}
}

func (v IntValue) Mul(other IntegerValue) IntegerValue {
	newValue := big.NewInt(0).Mul(v.Int, other.(IntValue).Int)
	return IntValue{newValue}
}

func (v IntValue) Div(other IntegerValue) IntegerValue {
	newValue := big.NewInt(0).Div(v.Int, other.(IntValue).Int)
	return IntValue{newValue}
}

func (v IntValue) Less(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return BoolValue(cmp == -1)
}

func (v IntValue) LessEqual(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return BoolValue(cmp <= 0)
}

func (v IntValue) Greater(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return BoolValue(cmp == 1)
}

func (v IntValue) GreaterEqual(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return BoolValue(cmp >= 0)
}

func (v IntValue) Equal(other IntegerValue) BoolValue {
	cmp := v.Int.Cmp(other.(IntValue).Int)
	return BoolValue(cmp == 0)
}

// Int8Value

type Int8Value int8

func (Int8Value) isValue() {}

func (v Int8Value) Copy() Value {
	return v
}

func (v Int8Value) ToGoValue() interface{} {
	return int8(v)
}

func (v Int8Value) IntValue() int {
	return int(v)
}

func (v Int8Value) Negate() IntegerValue {
	return -v
}

func (v Int8Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(Int8Value)
}

func (v Int8Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(Int8Value)
}

func (v Int8Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(Int8Value)
}

func (v Int8Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(Int8Value)
}

func (v Int8Value) Div(other IntegerValue) IntegerValue {
	return v / other.(Int8Value)
}

func (v Int8Value) Less(other IntegerValue) BoolValue {
	return v < other.(Int8Value)
}

func (v Int8Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(Int8Value)
}

func (v Int8Value) Greater(other IntegerValue) BoolValue {
	return v > other.(Int8Value)
}

func (v Int8Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(Int8Value)
}

func (v Int8Value) Equal(other IntegerValue) BoolValue {
	return v == other.(Int8Value)
}

// Int16Value

type Int16Value int16

func (Int16Value) isValue() {}

func (v Int16Value) Copy() Value {
	return v
}

func (v Int16Value) ToGoValue() interface{} {
	return int16(v)
}

func (v Int16Value) IntValue() int {
	return int(v)
}

func (v Int16Value) Negate() IntegerValue {
	return -v
}

func (v Int16Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(Int16Value)
}

func (v Int16Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(Int16Value)
}

func (v Int16Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(Int16Value)
}

func (v Int16Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(Int16Value)
}

func (v Int16Value) Div(other IntegerValue) IntegerValue {
	return v / other.(Int16Value)
}

func (v Int16Value) Less(other IntegerValue) BoolValue {
	return v < other.(Int16Value)
}

func (v Int16Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(Int16Value)
}

func (v Int16Value) Greater(other IntegerValue) BoolValue {
	return v > other.(Int16Value)
}

func (v Int16Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(Int16Value)
}

func (v Int16Value) Equal(other IntegerValue) BoolValue {
	return v == other.(Int16Value)
}

// Int32Value

type Int32Value int32

func (Int32Value) isValue() {}

func (v Int32Value) Copy() Value {
	return v
}

func (v Int32Value) ToGoValue() interface{} {
	return int32(v)
}

func (v Int32Value) IntValue() int {
	return int(v)
}

func (v Int32Value) Negate() IntegerValue {
	return -v
}

func (v Int32Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(Int32Value)
}

func (v Int32Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(Int32Value)
}

func (v Int32Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(Int32Value)
}

func (v Int32Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(Int32Value)
}

func (v Int32Value) Div(other IntegerValue) IntegerValue {
	return v / other.(Int32Value)
}

func (v Int32Value) Less(other IntegerValue) BoolValue {
	return v < other.(Int32Value)
}

func (v Int32Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(Int32Value)
}

func (v Int32Value) Greater(other IntegerValue) BoolValue {
	return v > other.(Int32Value)
}

func (v Int32Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(Int32Value)
}

func (v Int32Value) Equal(other IntegerValue) BoolValue {
	return v == other.(Int32Value)
}

// Int64Value

type Int64Value int64

func (Int64Value) isValue() {}

func (v Int64Value) Copy() Value {
	return v
}

func (v Int64Value) ToGoValue() interface{} {
	return int64(v)
}

func (v Int64Value) IntValue() int {
	return int(v)
}

func (v Int64Value) Negate() IntegerValue {
	return -v
}

func (v Int64Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(Int64Value)
}

func (v Int64Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(Int64Value)
}

func (v Int64Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(Int64Value)
}

func (v Int64Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(Int64Value)
}

func (v Int64Value) Div(other IntegerValue) IntegerValue {
	return v / other.(Int64Value)
}

func (v Int64Value) Less(other IntegerValue) BoolValue {
	return v < other.(Int64Value)
}

func (v Int64Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(Int64Value)
}

func (v Int64Value) Greater(other IntegerValue) BoolValue {
	return v > other.(Int64Value)
}

func (v Int64Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(Int64Value)
}

func (v Int64Value) Equal(other IntegerValue) BoolValue {
	return v == other.(Int64Value)
}

// UInt8Value

type UInt8Value uint8

func (UInt8Value) isValue() {}

func (v UInt8Value) Copy() Value {
	return v
}

func (v UInt8Value) ToGoValue() interface{} {
	return uint8(v)
}

func (v UInt8Value) IntValue() int {
	return int(v)
}

func (v UInt8Value) Negate() IntegerValue {
	return -v
}

func (v UInt8Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(UInt8Value)
}

func (v UInt8Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(UInt8Value)
}

func (v UInt8Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(UInt8Value)
}

func (v UInt8Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(UInt8Value)
}

func (v UInt8Value) Div(other IntegerValue) IntegerValue {
	return v / other.(UInt8Value)
}

func (v UInt8Value) Less(other IntegerValue) BoolValue {
	return v < other.(UInt8Value)
}

func (v UInt8Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(UInt8Value)
}

func (v UInt8Value) Greater(other IntegerValue) BoolValue {
	return v > other.(UInt8Value)
}

func (v UInt8Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(UInt8Value)
}

func (v UInt8Value) Equal(other IntegerValue) BoolValue {
	return v == other.(UInt8Value)
}

// UInt16Value

type UInt16Value uint16

func (UInt16Value) isValue() {}

func (v UInt16Value) Copy() Value {
	return v
}

func (v UInt16Value) ToGoValue() interface{} {
	return uint16(v)
}

func (v UInt16Value) IntValue() int {
	return int(v)
}
func (v UInt16Value) Negate() IntegerValue {
	return -v
}

func (v UInt16Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(UInt16Value)
}

func (v UInt16Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(UInt16Value)
}

func (v UInt16Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(UInt16Value)
}

func (v UInt16Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(UInt16Value)
}

func (v UInt16Value) Div(other IntegerValue) IntegerValue {
	return v / other.(UInt16Value)
}

func (v UInt16Value) Less(other IntegerValue) BoolValue {
	return v < other.(UInt16Value)
}

func (v UInt16Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(UInt16Value)
}

func (v UInt16Value) Greater(other IntegerValue) BoolValue {
	return v > other.(UInt16Value)
}

func (v UInt16Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(UInt16Value)
}

func (v UInt16Value) Equal(other IntegerValue) BoolValue {
	return v == other.(UInt16Value)
}

// UInt32Value

type UInt32Value uint32

func (UInt32Value) isValue() {}

func (v UInt32Value) Copy() Value {
	return v
}

func (v UInt32Value) ToGoValue() interface{} {
	return uint32(v)
}

func (v UInt32Value) IntValue() int {
	return int(v)
}

func (v UInt32Value) Negate() IntegerValue {
	return -v
}

func (v UInt32Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(UInt32Value)
}

func (v UInt32Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(UInt32Value)
}

func (v UInt32Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(UInt32Value)
}

func (v UInt32Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(UInt32Value)
}

func (v UInt32Value) Div(other IntegerValue) IntegerValue {
	return v / other.(UInt32Value)
}

func (v UInt32Value) Less(other IntegerValue) BoolValue {
	return v < other.(UInt32Value)
}

func (v UInt32Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(UInt32Value)
}

func (v UInt32Value) Greater(other IntegerValue) BoolValue {
	return v > other.(UInt32Value)
}

func (v UInt32Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(UInt32Value)
}

func (v UInt32Value) Equal(other IntegerValue) BoolValue {
	return v == other.(UInt32Value)
}

// UInt64Value

type UInt64Value uint64

func (UInt64Value) isValue() {}

func (v UInt64Value) Copy() Value {
	return v
}

func (v UInt64Value) ToGoValue() interface{} {
	return uint64(v)
}

func (v UInt64Value) IntValue() int {
	return int(v)
}

func (v UInt64Value) Negate() IntegerValue {
	return -v
}

func (v UInt64Value) Plus(other IntegerValue) IntegerValue {
	return v + other.(UInt64Value)
}

func (v UInt64Value) Minus(other IntegerValue) IntegerValue {
	return v - other.(UInt64Value)
}

func (v UInt64Value) Mod(other IntegerValue) IntegerValue {
	return v % other.(UInt64Value)
}

func (v UInt64Value) Mul(other IntegerValue) IntegerValue {
	return v * other.(UInt64Value)
}

func (v UInt64Value) Div(other IntegerValue) IntegerValue {
	return v / other.(UInt64Value)
}

func (v UInt64Value) Less(other IntegerValue) BoolValue {
	return v < other.(UInt64Value)
}

func (v UInt64Value) LessEqual(other IntegerValue) BoolValue {
	return v <= other.(UInt64Value)
}

func (v UInt64Value) Greater(other IntegerValue) BoolValue {
	return v > other.(UInt64Value)
}

func (v UInt64Value) GreaterEqual(other IntegerValue) BoolValue {
	return v >= other.(UInt64Value)
}

func (v UInt64Value) Equal(other IntegerValue) BoolValue {
	return v == other.(UInt64Value)
}

// StructureValue

type StructureValue struct {
	Identifier string
	Fields     *map[string]Value
	Functions  *map[string]FunctionValue
}

func (StructureValue) isValue() {}

func (v StructureValue) Copy() Value {
	newFields := make(map[string]Value, len(*v.Fields))
	for field, value := range *v.Fields {
		newFields[field] = value.Copy()
	}

	// NOTE: not copying functions – linked in

	return StructureValue{
		Identifier: v.Identifier,
		Fields:     &newFields,
		Functions:  v.Functions,
	}
}

func (v StructureValue) ToGoValue() interface{} {
	// TODO: convert values to Go values?
	return *v.Fields
}

func (v StructureValue) GetMember(interpreter *Interpreter, name string) Value {
	value, ok := (*v.Fields)[name]
	if ok {
		return value
	}

	// if structure was deserialized, dynamically link in the functions
	if v.Functions == nil {
		functions := interpreter.StructureFunctions[v.Identifier]
		v.Functions = &functions
	}

	function, ok := (*v.Functions)[name]
	if ok {
		if interpretedFunction, ok := function.(InterpretedFunctionValue); ok {
			function = interpreter.bindSelf(interpretedFunction, v)
		}
		return function
	}

	return nil
}

func (v StructureValue) SetMember(interpreter *Interpreter, name string, value Value) {
	(*v.Fields)[name] = value
}

func (v StructureValue) GobEncode() ([]byte, error) {
	w := new(bytes.Buffer)
	encoder := gob.NewEncoder(w)
	err := encoder.Encode(v.Identifier)
	if err != nil {
		return nil, err
	}
	err = encoder.Encode(v.Fields)
	if err != nil {
		return nil, err
	}
	// NOTE: *not* encoding functions – linked in on-demand
	return w.Bytes(), nil
}

func (v *StructureValue) GobDecode(buf []byte) error {
	r := bytes.NewBuffer(buf)
	decoder := gob.NewDecoder(r)
	err := decoder.Decode(&v.Identifier)
	if err != nil {
		return err
	}
	err = decoder.Decode(&v.Fields)
	if err != nil {
		return err
	}
	// NOTE: *not* decoding functions – linked in on-demand
	return nil
}

// DictionaryValue

type DictionaryValue map[interface{}]Value

func (DictionaryValue) isValue() {}

func (v DictionaryValue) Copy() Value {
	newDictionary := make(DictionaryValue, len(v))
	for field, value := range v {
		newDictionary[field] = value.Copy()
	}
	return newDictionary
}

func (v DictionaryValue) ToGoValue() interface{} {
	// TODO: convert values to Go values?
	return v
}

func (v DictionaryValue) isIndexableValue() {}

func (v DictionaryValue) Get(keyValue Value) Value {
	value, ok := v[dictionaryKey(keyValue)]
	if !ok {
		return NilValue{}
	}
	return SomeValue{Value: value}
}

func dictionaryKey(keyValue Value) interface{} {
	var key interface{} = keyValue
	if keyValue, ok := keyValue.(HasKeyString); ok {
		return keyValue.KeyString()
	}
	return key
}

type HasKeyString interface {
	KeyString() string
}

func (v DictionaryValue) Set(keyValue Value, value Value) {
	v[dictionaryKey(keyValue)] = value
}

func (v DictionaryValue) String() string {
	var builder strings.Builder
	builder.WriteString("{")
	i := 0
	for key, value := range v {
		if i > 0 {
			builder.WriteString(", ")
		}
		builder.WriteString(fmt.Sprint(key))
		builder.WriteString(": ")
		builder.WriteString(fmt.Sprint(value))
		i += 1
	}
	builder.WriteString("}")
	return builder.String()
}

func (v DictionaryValue) GetMember(interpreter *Interpreter, name string) Value {
	switch name {
	case "length":
		return IntValue{Int: big.NewInt(int64(len(v)))}
	case "remove":
		return NewHostFunctionValue(
			func(arguments []Value, location Location) trampoline.Trampoline {
				key := dictionaryKey(arguments[0])
				value, ok := v[key]
				if !ok {
					return trampoline.Done{Result: NilValue{}}
				}
				delete(v, key)
				return trampoline.Done{Result: SomeValue{Value: value}}
			},
		)
	default:
		panic(&errors.UnreachableError{})
	}
}

func (v DictionaryValue) SetMember(interpreter *Interpreter, name string, value Value) {
	panic(&errors.UnreachableError{})
}

// ToValue

// ToValue converts a Go value into an interpreter value
func ToValue(value interface{}) (Value, error) {
	// TODO: support more types
	switch value := value.(type) {
	case *big.Int:
		return IntValue{value}, nil
	case int8:
		return Int8Value(value), nil
	case int16:
		return Int16Value(value), nil
	case int32:
		return Int32Value(value), nil
	case int64:
		return Int64Value(value), nil
	case uint8:
		return UInt8Value(value), nil
	case uint16:
		return UInt16Value(value), nil
	case uint32:
		return UInt32Value(value), nil
	case uint64:
		return UInt64Value(value), nil
	case bool:
		return BoolValue(value), nil
	case string:
		return StringValue(value), nil
	case nil:
		return NilValue{}, nil
	}

	return nil, fmt.Errorf("cannot convert Go value to value: %#+v", value)
}

func ToValues(inputs []interface{}) ([]Value, error) {
	var values []Value
	for _, argument := range inputs {
		value, ok := argument.(Value)
		if !ok {
			var err error
			value, err = ToValue(argument)
			if err != nil {
				return nil, err
			}
		}
		values = append(
			values,
			value,
		)
	}
	return values, nil
}

// NilValue

type NilValue struct{}

func (NilValue) isValue() {}

func (v NilValue) Copy() Value {
	return v
}

func (NilValue) String() string {
	return "nil"
}

func (v NilValue) ToGoValue() interface{} {
	return nil
}

// SomeValue

type SomeValue struct {
	Value Value
}

func (SomeValue) isValue() {}

func (v SomeValue) Copy() Value {
	return SomeValue{
		Value: v.Value.Copy(),
	}
}

func (v SomeValue) String() string {
	return fmt.Sprint(v.Value)
}

// MetaTypeValue

type MetaTypeValue struct {
	Type sema.Type
}

func (MetaTypeValue) isValue() {}

func (v MetaTypeValue) Copy() Value {
	return v
}

// AnyValue

type AnyValue struct {
	Value Value
	Type  sema.Type
}

func (AnyValue) isValue() {}

func (v AnyValue) Copy() Value {
	return AnyValue{
		Value: v.Value.Copy(),
		Type:  v.Type,
	}
}

func (v AnyValue) String() string {
	return fmt.Sprint(v.Value)
}

// ValueWithMembers

type ValueWithMembers interface {
	GetMember(interpreter *Interpreter, name string) Value
	SetMember(interpreter *Interpreter, name string, value Value)
}

//

func init() {
	gob.Register(VoidValue{})
	gob.Register(BoolValue(true))
	gob.Register(StringValue(""))
	gob.Register(ArrayValue{})
	gob.Register(IntValue{})
	gob.Register(Int8Value(0))
	gob.Register(Int16Value(0))
	gob.Register(Int32Value(0))
	gob.Register(Int64Value(0))
	gob.Register(UInt8Value(0))
	gob.Register(UInt16Value(0))
	gob.Register(UInt32Value(0))
	gob.Register(UInt64Value(0))
	gob.Register(StructureValue{})
	gob.Register(DictionaryValue{})
	gob.Register(NilValue{})
	gob.Register(SomeValue{})
	gob.Register(MetaTypeValue{})
	gob.Register(AnyValue{})
}

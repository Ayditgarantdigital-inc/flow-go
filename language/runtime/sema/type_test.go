package sema

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/language/runtime/common"
)

func TestConstantSizedType_String(t *testing.T) {

	ty := &ConstantSizedType{
		Type: &VariableSizedType{Type: &IntType{}},
		Size: 2,
	}

	assert.Equal(t, ty.String(), "[[Int]; 2]")
}

func TestConstantSizedType_String_OfFunctionType(t *testing.T) {

	ty := &ConstantSizedType{
		Type: &FunctionType{
			Parameters: []*Parameter{
				{
					TypeAnnotation: NewTypeAnnotation(&Int8Type{}),
				},
			},
			ReturnTypeAnnotation: NewTypeAnnotation(
				&Int16Type{},
			),
		},
		Size: 2,
	}

	assert.Equal(t, ty.String(), "[((Int8): Int16); 2]")
}

func TestVariableSizedType_String(t *testing.T) {

	ty := &VariableSizedType{
		Type: &ConstantSizedType{
			Type: &IntType{},
			Size: 2,
		},
	}

	assert.Equal(t, ty.String(), "[[Int; 2]]")
}

func TestVariableSizedType_String_OfFunctionType(t *testing.T) {

	ty := &VariableSizedType{
		Type: &FunctionType{
			Parameters: []*Parameter{
				{
					TypeAnnotation: NewTypeAnnotation(&Int8Type{}),
				},
			},
			ReturnTypeAnnotation: NewTypeAnnotation(
				&Int16Type{},
			),
		},
	}

	assert.Equal(t, ty.String(), "[((Int8): Int16)]")
}

func TestIsResourceType_AnyStructNestedInArray(t *testing.T) {

	ty := &VariableSizedType{
		Type: &AnyStructType{},
	}

	assert.False(t, ty.IsResourceType())
}

func TestIsResourceType_AnyResourceNestedInArray(t *testing.T) {

	ty := &VariableSizedType{
		Type: &AnyResourceType{},
	}

	assert.True(t, ty.IsResourceType())
}

func TestIsResourceType_ResourceNestedInArray(t *testing.T) {

	ty := &VariableSizedType{
		Type: &CompositeType{
			Kind: common.CompositeKindResource,
		},
	}

	assert.True(t, ty.IsResourceType())
}

func TestIsResourceType_ResourceNestedInDictionary(t *testing.T) {

	ty := &DictionaryType{
		KeyType: &StringType{},
		ValueType: &VariableSizedType{
			Type: &CompositeType{
				Kind: common.CompositeKindResource,
			},
		},
	}

	assert.True(t, ty.IsResourceType())
}

func TestIsResourceType_StructNestedInDictionary(t *testing.T) {

	ty := &DictionaryType{
		KeyType: &StringType{},
		ValueType: &VariableSizedType{
			Type: &CompositeType{
				Kind: common.CompositeKindStructure,
			},
		},
	}

	assert.False(t, ty.IsResourceType())
}

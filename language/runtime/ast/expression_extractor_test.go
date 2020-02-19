package ast

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testIntExtractor struct{}

func (testIntExtractor) ExtractInteger(
	extractor *ExpressionExtractor,
	expression *IntegerExpression,
) ExpressionExtraction {

	newIdentifier := Identifier{
		Identifier: extractor.FreshIdentifier(),
	}
	newExpression := &IdentifierExpression{
		Identifier: newIdentifier,
	}
	return ExpressionExtraction{
		RewrittenExpression: newExpression,
		ExtractedExpressions: []ExtractedExpression{
			{
				Identifier: newIdentifier,
				Expression: expression,
			},
		},
	}
}

func TestExpressionExtractorBinaryExpressionNothingExtracted(t *testing.T) {

	expression := &BinaryExpression{
		Operation: OperationEqual,
		Left: &IdentifierExpression{
			Identifier: Identifier{Identifier: "x"},
		},
		Right: &IdentifierExpression{
			Identifier: Identifier{Identifier: "y"},
		},
	}

	extractor := &ExpressionExtractor{
		IntExtractor: testIntExtractor{},
	}

	result := extractor.Extract(expression)

	assert.Equal(t,
		result,
		ExpressionExtraction{
			RewrittenExpression: &BinaryExpression{
				Operation: OperationEqual,
				Left: &IdentifierExpression{
					Identifier{Identifier: "x"},
				},
				Right: &IdentifierExpression{
					Identifier{Identifier: "y"},
				},
			},
			ExtractedExpressions: nil,
		},
	)
}

func TestExpressionExtractorBinaryExpressionIntegerExtracted(t *testing.T) {

	expression := &BinaryExpression{
		Operation: OperationEqual,
		Left: &IdentifierExpression{
			Identifier{Identifier: "x"},
		},
		Right: &IntegerExpression{
			Value: big.NewInt(1),
			Base:  10,
		},
	}

	extractor := &ExpressionExtractor{
		IntExtractor: testIntExtractor{},
	}

	result := extractor.Extract(expression)

	newIdentifier := extractor.FormatIdentifier(0)

	assert.Equal(t,
		result,
		ExpressionExtraction{
			RewrittenExpression: &BinaryExpression{
				Operation: OperationEqual,
				Left: &IdentifierExpression{
					Identifier{Identifier: "x"},
				},
				Right: &IdentifierExpression{
					Identifier{Identifier: newIdentifier},
				},
			},
			ExtractedExpressions: []ExtractedExpression{
				{
					Identifier: Identifier{Identifier: newIdentifier},
					Expression: &IntegerExpression{
						Value: big.NewInt(1),
						Base:  10,
					},
				},
			},
		},
	)
}

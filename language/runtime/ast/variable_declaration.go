package ast

import "github.com/dapperlabs/flow-go/language/runtime/common"

type VariableDeclaration struct {
	Access            Access
	IsConstant        bool
	Identifier        Identifier
	TypeAnnotation    *TypeAnnotation
	Value             Expression
	Transfer          *Transfer
	StartPos          Position
	SecondTransfer    *Transfer
	SecondValue       Expression
	ParentIfStatement *IfStatement
}

func (v *VariableDeclaration) StartPosition() Position {
	return v.StartPos
}

func (v *VariableDeclaration) EndPosition() Position {
	return v.Value.EndPosition()
}

func (*VariableDeclaration) isIfStatementTest() {}

func (*VariableDeclaration) isDeclaration() {}

func (*VariableDeclaration) isStatement() {}

func (v *VariableDeclaration) Accept(visitor Visitor) Repr {
	return visitor.VisitVariableDeclaration(v)
}

func (v *VariableDeclaration) DeclarationName() string {
	return v.Identifier.Identifier
}

func (v *VariableDeclaration) DeclarationKind() common.DeclarationKind {
	if v.IsConstant {
		return common.DeclarationKindConstant
	}
	return common.DeclarationKindVariable
}

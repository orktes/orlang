package ast

import (
	"fmt"

	"github.com/orktes/orlang/scanner"
)

// CastExpression represents a type cast (e.g., value as Type)
type CastExpression struct {
	Left  Expression    // The value being cast
	Token scanner.Token // The 'as' token
	Type  Type          // The target type
}

func (CastExpression) exprNode() {}

func (ce *CastExpression) StartPos() Position {
	return ce.Left.StartPos()
}

func (ce *CastExpression) EndPos() Position {
	return ce.Type.EndPos()
}

func (ce *CastExpression) String() string {
	return fmt.Sprintf("%v as %v", ce.Left, ce.Type)
}

package ast

import (
	"fmt"

	"github.com/orktes/orlang/scanner"
)

// TypeAssertionExpression represents a type assertion (e.g., value is Type)
type TypeAssertionExpression struct {
	Expression Expression    // The value being checked
	IsToken    scanner.Token // The 'is' keyword token
	Type       Type          // The type to check against
}

func (TypeAssertionExpression) exprNode() {}

func (ta *TypeAssertionExpression) StartPos() Position {
	return ta.Expression.StartPos()
}

func (ta *TypeAssertionExpression) EndPos() Position {
	return ta.Type.EndPos()
}

func (ta *TypeAssertionExpression) String() string {
	return fmt.Sprintf("%v is %v", ta.Expression, ta.Type)
}

package ast

import (
	"fmt"

	"github.com/orktes/orlang/scanner"
)

type ComparisonExpression struct {
	Operator scanner.Token
	Left     Expression
	Right    Expression
}

func (a *ComparisonExpression) StartPos() Position {
	return a.Left.StartPos()
}

func (a *ComparisonExpression) EndPos() Position {
	return a.Right.EndPos()
}

func (_ *ComparisonExpression) exprNode() {}

func (b *ComparisonExpression) String() string {
	return fmt.Sprintf("%s %s %s", b.Left, b.Operator.Text, b.Right)
}

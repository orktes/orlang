package ast

import (
	"fmt"

	"github.com/orktes/orlang/scanner"
)

type BinaryExpression struct {
	Left     Expression
	Right    Expression
	Operator scanner.Token
}

func (b *BinaryExpression) StartPos() Position {
	return b.Left.StartPos()
}

func (b *BinaryExpression) EndPos() Position {
	return b.Right.EndPos()
}

func (_ *BinaryExpression) exprNode() {}

func (b *BinaryExpression) String() string {
	return fmt.Sprintf("(%s %s %s)", b.Left, b.Operator.Text, b.Right)
}

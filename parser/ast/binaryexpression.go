package ast

import "github.com/orktes/orlang/parser/scanner"

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

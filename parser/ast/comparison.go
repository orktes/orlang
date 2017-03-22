package ast

import "github.com/orktes/orlang/parser/scanner"

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

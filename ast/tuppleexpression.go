package ast

import "github.com/orktes/orlang/scanner"

type TuppleExpression struct {
	LeftParen   scanner.Token
	Expressions []Expression
	RightParen  scanner.Token
}

func (TuppleExpression) exprNode() {}

func (te *TuppleExpression) StartPos() Position {
	return StartPositionFromToken(te.LeftParen)
}

func (te *TuppleExpression) EndPos() Position {
	return EndPositionFromToken(te.RightParen)
}

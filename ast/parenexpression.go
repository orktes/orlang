package ast

import "github.com/orktes/orlang/scanner"

type ParenExpression struct {
	LeftParen  scanner.Token
	Expression Expression
	RightParen scanner.Token
}

func (ParenExpression) exprNode() {}

func (pe *ParenExpression) StartPos() Position {
	return StartPositionFromToken(pe.LeftParen)
}

func (pe *ParenExpression) EndPos() Position {
	return EndPositionFromToken(pe.RightParen)
}

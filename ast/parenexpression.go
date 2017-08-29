package ast

import (
	"fmt"

	"github.com/orktes/orlang/scanner"
)

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

func (pe *ParenExpression) String() string {
	return fmt.Sprintf("%s", pe.Expression)
}

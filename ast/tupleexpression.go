package ast

import (
	"fmt"
	"strings"

	"github.com/orktes/orlang/scanner"
)

type TupleExpression struct {
	LeftParen   scanner.Token
	Expressions []Expression
	RightParen  scanner.Token
}

func (TupleExpression) exprNode() {}

func (te *TupleExpression) StartPos() Position {
	return StartPositionFromToken(te.LeftParen)
}

func (te *TupleExpression) EndPos() Position {
	return EndPositionFromToken(te.RightParen)
}

func (te *TupleExpression) String() string {
	names := []string{}

	for _, expr := range te.Expressions {
		names = append(names, fmt.Sprintf("%s", expr))
	}

	return fmt.Sprintf("(%s)", strings.Join(names, ", "))
}

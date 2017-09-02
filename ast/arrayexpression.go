package ast

import (
	"fmt"
	"strings"

	"github.com/orktes/orlang/scanner"
)

type ArrayExpression struct {
	Type        *ArrayType
	Expressions []Expression
	LeftBrace   scanner.Token
	RightBrace  scanner.Token
}

func (ArrayExpression) exprNode() {}

func (ae *ArrayExpression) StartPos() Position {
	return ae.Type.StartPos()
}

func (ae *ArrayExpression) EndPos() Position {
	return EndPositionFromToken(ae.RightBrace)
}

func (ae *ArrayExpression) String() string {
	names := []string{}

	for _, expr := range ae.Expressions {
		names = append(names, fmt.Sprintf("%s", expr))
	}

	return fmt.Sprintf("%s{%s}", ae.Type, strings.Join(names, ", "))
}

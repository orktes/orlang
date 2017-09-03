package ast

import (
	"fmt"
	"strings"

	"github.com/orktes/orlang/scanner"
)

type TuplePattern struct {
	Patterns   []Pattern
	LeftParen  scanner.Token
	RightParen scanner.Token
}

func (TuplePattern) patternNode()     {}
func (TuplePattern) declarationNode() {}

func (tp *TuplePattern) StartPos() Position {
	return StartPositionFromToken(tp.LeftParen)
}

func (tp *TuplePattern) EndPos() Position {
	return EndPositionFromToken(tp.RightParen)
}

func (tp *TuplePattern) String() string {
	names := []string{}

	for _, expr := range tp.Patterns {
		names = append(names, fmt.Sprintf("%s", expr))
	}

	return fmt.Sprintf("(%s)", strings.Join(names, ", "))
}

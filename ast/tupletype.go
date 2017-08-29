package ast

import "github.com/orktes/orlang/scanner"

type TupleType struct {
	LeftParen  scanner.Token
	Types      []Type
	RightParen scanner.Token
}

func (TupleType) typeNode() {}

func (tt *TupleType) StartPos() Position {
	return StartPositionFromToken(tt.LeftParen)
}

func (tt *TupleType) EndPos() Position {
	return EndPositionFromToken(tt.RightParen)
}

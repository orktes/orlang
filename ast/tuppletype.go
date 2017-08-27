package ast

import "github.com/orktes/orlang/scanner"

type TuppleType struct {
	LeftParen  scanner.Token
	Types      []Type
	RightParen scanner.Token
}

func (TuppleType) typeNode() {}

func (tt *TuppleType) StartPos() Position {
	return StartPositionFromToken(tt.LeftParen)
}

func (tt *TuppleType) EndPos() Position {
	return EndPositionFromToken(tt.RightParen)
}

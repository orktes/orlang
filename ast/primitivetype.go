package ast

import "github.com/orktes/orlang/scanner"

type PrimitiveType struct {
	Token scanner.Token
}

func (pt *PrimitiveType) StartPos() Position {
	return StartPositionFromToken(pt.Token)
}

func (pt *PrimitiveType) EndPos() Position {
	return EndPositionFromToken(pt.Token)
}

func (_ *PrimitiveType) typeNode() {}

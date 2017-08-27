package ast

import "github.com/orktes/orlang/scanner"

type TypeReference struct {
	Token scanner.Token
}

func (pt *TypeReference) StartPos() Position {
	return StartPositionFromToken(pt.Token)
}

func (pt *TypeReference) EndPos() Position {
	return EndPositionFromToken(pt.Token)
}

func (_ *TypeReference) typeNode() {}

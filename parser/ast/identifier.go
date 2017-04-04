package ast

import "github.com/orktes/orlang/parser/scanner"

// Identifier temp container for direct values
type Identifier struct {
	scanner.Token
}

func (i *Identifier) StartPos() Position {
	return StartPositionFromToken(i.Token)
}

func (i *Identifier) EndPos() Position {
	return EndPositionFromToken(i.Token)
}

func (_ *Identifier) exprNode() {
}

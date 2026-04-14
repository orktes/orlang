package ast

import "github.com/orktes/orlang/scanner"

// PointerType represents a pointer type (&type)
type PointerType struct {
	Ampersand scanner.Token // The & token
	Type      Type          // The pointed-to type
}

func (PointerType) typeNode() {}

func (p *PointerType) StartPos() Position {
	return StartPositionFromToken(p.Ampersand)
}

func (p *PointerType) EndPos() Position {
	if p.Type != nil {
		return p.Type.EndPos()
	}
	return EndPositionFromToken(p.Ampersand)
}

package ast

import "github.com/orktes/orlang/parser/scanner"

type FunctionDeclaration struct {
	Start       Position
	Name        scanner.Token
	Arguments   []*Argument
	ReturnTypes []*Argument
	Block       *Block
}

func (fd *FunctionDeclaration) StartPos() Position {
	return fd.Start
}

func (fd *FunctionDeclaration) EndPos() Position {
	if fd.Block == nil {
		return fd.Start
	}
	return fd.Block.End
}

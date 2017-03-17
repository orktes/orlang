package ast

import "github.com/orktes/orlang/parser/scanner"

type Argument struct {
	Name         scanner.Token
	Type         scanner.Token
	DefaultValue Expression
}

func (a *Argument) StartPos() Position {
	return StartPositionFromToken(a.Name)
}

func (a *Argument) EndPos() Position {
	if a.DefaultValue != nil {
		return a.DefaultValue.EndPos()
	}
	return StartPositionFromToken(a.Type)
}

type FunctionDeclaration struct {
	Start       Position
	Name        scanner.Token
	Arguments   []Argument
	ReturnTypes []Argument
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

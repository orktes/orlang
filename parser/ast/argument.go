package ast

import "github.com/orktes/orlang/parser/scanner"

type Argument struct {
	Name         *Identifier
	Type         *scanner.Token
	DefaultValue Expression
	Variadic     bool
}

func (a *Argument) StartPos() Position {
	return a.Name.StartPos()
}

func (a *Argument) EndPos() Position {
	if a.DefaultValue != nil {
		return a.DefaultValue.EndPos()
	}
	if a.Type != nil {
		return EndPositionFromToken(*a.Type)
	}

	return a.EndPos()
}

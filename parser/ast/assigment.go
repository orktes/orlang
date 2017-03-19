package ast

import "github.com/orktes/orlang/parser/scanner"

type Assigment struct {
	Identifier scanner.Token
	Expression Expression
}

func (a *Assigment) StartPos() Position {
	return StartPositionFromToken(a.Identifier)
}

func (a *Assigment) EndPos() Position {
	return a.Expression.EndPos()
}

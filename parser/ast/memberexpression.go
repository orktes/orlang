package ast

import "github.com/orktes/orlang/parser/scanner"

type MemberExpression struct {
	Target   Expression
	Property scanner.Token
}

func (me *MemberExpression) StartPos() Position {
	return me.StartPos()
}

func (me *MemberExpression) EndPos() Position {
	return EndPositionFromToken(me.Property)
}
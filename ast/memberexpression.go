package ast

import "github.com/orktes/orlang/scanner"

type MemberExpression struct {
	Target   Expression
	Property scanner.Token
}

func (me *MemberExpression) StartPos() Position {
	return me.Target.StartPos()
}

func (me *MemberExpression) EndPos() Position {
	return EndPositionFromToken(me.Property)
}

func (_ *MemberExpression) exprNode() {
}

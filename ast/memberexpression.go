package ast

import (
	"fmt"

	"github.com/orktes/orlang/scanner"
)

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

func (me *MemberExpression) String() string {
	return fmt.Sprintf("%s.%s", me.Target, me.Property.Text)
}

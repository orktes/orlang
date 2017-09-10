package ast

import "fmt"

type MemberExpression struct {
	Target   Expression
	Property *Identifier
}

func (me *MemberExpression) StartPos() Position {
	return me.Target.StartPos()
}

func (me *MemberExpression) EndPos() Position {
	return me.Property.EndPos()
}

func (_ *MemberExpression) exprNode() {
}

func (me *MemberExpression) String() string {
	return fmt.Sprintf("%s.%s", me.Target, me.Property.Text)
}

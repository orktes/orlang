package ast

import "fmt"

type ReturnStatement struct {
	Start      Position
	ReturnEnd  Position
	Expression Expression
}

func (rtrn *ReturnStatement) StartPos() Position {
	return rtrn.Start
}

func (rtrn *ReturnStatement) EndPos() Position {
	if rtrn.Expression == nil {
		return rtrn.ReturnEnd
	}
	return rtrn.Expression.EndPos()
}

func (rtrn *ReturnStatement) String() string {
	if rtrn.Expression != nil {
		return fmt.Sprintf("return %s", rtrn.Expression)
	}
	return "return"
}

func (_ *ReturnStatement) stmtNode() {}

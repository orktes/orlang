package ast

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

func (_ *ReturnStatement) stmtNode() {}

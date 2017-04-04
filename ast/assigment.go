package ast

type Assigment struct {
	Left  Expression
	Right Expression
}

func (a *Assigment) StartPos() Position {
	return a.Left.StartPos()
}

func (a *Assigment) EndPos() Position {
	return a.Right.EndPos()
}

func (_ *Assigment) stmtNode() {}
func (_ *Assigment) exprNode() {}

package ast

// GoStatement runs a function call in a new green thread: go f(x, y)
type GoStatement struct {
	Start Position
	Call  Expression // Must be a *FunctionCall
}

func (g *GoStatement) StartPos() Position {
	return g.Start
}

func (g *GoStatement) EndPos() Position {
	return g.Call.EndPos()
}

func (_ *GoStatement) stmtNode() {}

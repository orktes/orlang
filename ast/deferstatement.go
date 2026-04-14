package ast

type DeferStatement struct {
	Start    Position
	DeferEnd Position
	Call     Expression // Must be a *FunctionCall
}

func (d *DeferStatement) StartPos() Position {
	return d.Start
}

func (d *DeferStatement) EndPos() Position {
	return d.Call.EndPos()
}

func (_ *DeferStatement) stmtNode() {}

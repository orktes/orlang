package ast

type FunctionCall struct {
	Callee Expression
	End    Position
}

func (fc *FunctionCall) StartPos() Position {
	return fc.Callee.StartPos()
}

func (fc *FunctionCall) EndPos() Position {
	return fc.End
}
package ast

type Interface struct {
	Start     Position
	Name      *Identifier
	Functions []*FunctionSignature
	End       Position
}

func (i *Interface) StartPos() Position {
	return i.Start
}

func (i *Interface) EndPos() Position {
	return i.End
}

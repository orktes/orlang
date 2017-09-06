package ast

type Struct struct {
	Start     Position
	Name      *Identifier
	Variables []*VariableDeclaration
	Functions []*FunctionDeclaration
	End       Position
}

func (sd *Struct) StartPos() Position {
	return sd.Start
}

func (sd *Struct) EndPos() Position {
	return sd.End
}

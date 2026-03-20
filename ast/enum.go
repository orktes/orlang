package ast

type EnumValue struct {
	Name *Identifier
}

type Enum struct {
	Start  Position
	Name   *Identifier
	Values []*EnumValue
	End    Position
}

func (e *Enum) StartPos() Position {
	return e.Start
}

func (e *Enum) EndPos() Position {
	return e.End
}

func (_ *Enum) stmtNode() {}

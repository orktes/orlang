package ast

type Argument struct {
	Name         *Identifier
	Type         Type
	DefaultValue Expression
	Variadic     bool
}

func (a *Argument) StartPos() Position {
	return a.Name.StartPos()
}

func (a *Argument) EndPos() Position {
	if a.DefaultValue != nil {
		return a.DefaultValue.EndPos()
	}
	if a.Type != nil {
		return a.Type.EndPos()
	}

	return a.EndPos()
}

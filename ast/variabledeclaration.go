package ast

type VariableDeclaration struct {
	Name         *Identifier
	Type         Type
	DefaultValue Expression
	Constant     bool
}

func (vd *VariableDeclaration) StartPos() Position {
	return vd.Name.StartPos()
}

func (vd *VariableDeclaration) EndPos() Position {
	if vd.DefaultValue != nil {
		return vd.DefaultValue.EndPos()
	}
	return vd.Type.EndPos()
}

func (_ *VariableDeclaration) stmtNode() {}

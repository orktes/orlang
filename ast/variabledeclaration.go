package ast

type VariableDeclaration struct {
	Name         *Identifier
	Type         Type
	DefaultValue Expression
	Constant     bool
}

func (*VariableDeclaration) declarationNode() {}

func (vd *VariableDeclaration) GetIdentifier() *Identifier {
	return vd.Name
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

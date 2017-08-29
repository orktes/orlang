package ast

type TupleDeclaration struct {
	Pattern      *TuplePattern
	Type         Type
	DefaultValue Expression
	Constant     bool
}

func (*TupleDeclaration) declarationNode() {}
func (*TupleDeclaration) stmtNode()        {}

func (vd *TupleDeclaration) GetIdentifier() *Identifier {
	return nil
}

func (vd *TupleDeclaration) StartPos() Position {
	return vd.Pattern.StartPos()
}

func (vd *TupleDeclaration) EndPos() Position {
	if vd.DefaultValue != nil {
		return vd.DefaultValue.EndPos()
	}
	return vd.Type.EndPos()
}

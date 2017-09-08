package ast

type StructExpression struct {
	Identifier *Identifier
	End        Position
	// TODO properties
}

func (StructExpression) exprNode() {}

func (se *StructExpression) StartPos() Position {
	return se.Identifier.StartPos()
}

func (se *StructExpression) EndPos() Position {
	return se.End
}

func (se *StructExpression) String() string {
	return "struct STRING TODO"
}

package ast

type StructExpression struct {
	Identifier *Identifier
	End        Position
	Arguments  []*CallArgument
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

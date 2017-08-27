package ast

type CallArgument struct {
	Name       *Identifier
	Expression Expression
}

func (ca *CallArgument) StartPos() Position {
	if ca.Name != nil {
		return ca.Name.StartPos()
	}
	return ca.Expression.StartPos()
}

func (ca *CallArgument) EndPos() Position {
	return ca.Expression.EndPos()
}

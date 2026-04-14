package ast

type IncludeStatement struct {
	Path *ValueExpression
}

func (n *IncludeStatement) StartPos() Position {
	return n.Path.StartPos()
}

func (n *IncludeStatement) EndPos() Position {
	return n.Path.EndPos()
}

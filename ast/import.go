package ast

type ImportStatement struct {
	Imports []*Identifier
	Path    *ValueExpression
}

func (n *ImportStatement) StartPos() Position {
	return n.Imports[0].StartPos()
}

func (n *ImportStatement) EndPos() Position {
	return n.Path.EndPos()
}

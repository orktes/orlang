package ast

type ImportItem struct {
	Name  *Identifier // The imported name
	Alias *Identifier // Optional alias (can be nil)
}

type ImportStatement struct {
	Items []*ImportItem
	Path  *ValueExpression
}

func (n *ImportStatement) StartPos() Position {
	if len(n.Items) > 0 {
		return n.Items[0].Name.StartPos()
	}
	return n.Path.StartPos()
}

func (n *ImportStatement) EndPos() Position {
	return n.Path.EndPos()
}

package ast

type ExportStatement struct {
	Declaration Node
}

func (n *ExportStatement) StartPos() Position {
	return n.Declaration.StartPos()
}

func (n *ExportStatement) EndPos() Position {
	return n.Declaration.EndPos()
}

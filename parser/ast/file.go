package ast

type File struct {
	Body         []Node
	NodeComments map[Node][]Comment
	Comments     []Comment
	Macros       map[string]*Macro
}

func (f *File) AppendNode(node Node) {
	f.Body = append(f.Body, node)
}

func (f *File) StartPos() Position {
	return Position{Line: 0, Column: 0}
}

func (f *File) EndPos() Position {
	if len(f.Body) > 0 {
		return f.Body[len(f.Body)-1].EndPos()
	}

	return Position{Line: 0, Column: 0}
}

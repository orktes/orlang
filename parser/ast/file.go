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

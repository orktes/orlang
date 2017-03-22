package ast

type File struct {
	Body         []Node
	NodeComments map[Node][]Comment
	Comments     []Comment
}

func (f *File) AppendNode(node Node) {
	f.Body = append(f.Body, node)
}

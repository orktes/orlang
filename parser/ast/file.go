package ast

type File struct {
	Body []Node
}

func (f *File) AppendNode(node Node) {
	f.Body = append(f.Body, node)
}

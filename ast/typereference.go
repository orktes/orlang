package ast

type TypeReference struct {
	Name *Identifier
}

func (pt *TypeReference) StartPos() Position {
	return pt.Name.StartPos()
}

func (pt *TypeReference) EndPos() Position {
	return pt.Name.EndPos()
}

func (pt *TypeReference) String() string {
	return pt.Name.Text
}

func (_ *TypeReference) typeNode() {}

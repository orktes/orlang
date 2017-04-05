package ast

type Type interface {
	Node
	typeNode()
}

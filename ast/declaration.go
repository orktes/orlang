package ast

type Declaration interface {
	Node
	declarationNode()
}

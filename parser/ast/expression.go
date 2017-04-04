package ast

type Expression interface {
	Node
	exprNode()
}

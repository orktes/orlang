package ast

type Statement interface {
	Node
	stmtNode()
}

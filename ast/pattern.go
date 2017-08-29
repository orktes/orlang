package ast

type Pattern interface {
	Node
	patternNode()
}

package ast

import "github.com/orktes/orlang/scanner"

// LinkKind distinguishes the three forms of link directive.
type LinkKind int

const (
	LinkKindFile LinkKind = iota // link "file.c"
	LinkKindPkg                  // link pkg "name"
	LinkKindLib                  // link lib "name"
)

type LinkStatement struct {
	LinkToken scanner.Token
	Kind      LinkKind
	Path      *ValueExpression
}

func (n *LinkStatement) StartPos() Position {
	return StartPositionFromToken(n.LinkToken)
}

func (n *LinkStatement) EndPos() Position {
	return n.Path.EndPos()
}

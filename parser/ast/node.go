package ast

import "github.com/orktes/orlang/parser/scanner"

type Position struct {
	Line   int
	Column int
}

type Node interface {
	StartPos() Position
	EndPos() Position
}

func StartPositionFromToken(token scanner.Token) Position {
	return Position{Line: token.StartLine, Column: token.StartColumn}
}

func EndPositionFromToken(token scanner.Token) Position {
	return Position{Line: token.EndLine, Column: token.EndColumn}
}

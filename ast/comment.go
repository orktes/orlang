package ast

import "github.com/orktes/orlang/scanner"

type Comment struct {
	Token scanner.Token
}

func (c *Comment) StartPos() Position {
	return StartPositionFromToken(c.Token)
}

func (c *Comment) EndPos() Position {
	return EndPositionFromToken(c.Token)
}

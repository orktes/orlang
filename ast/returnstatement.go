package ast

import "github.com/orktes/orlang/scanner"

type ReturnStatement struct {
	scanner.Token
	Expression Expression
}

func (rtrn *ReturnStatement) StartPos() Position {
	return StartPositionFromToken(rtrn.Token)
}

func (rtrn *ReturnStatement) EndPos() Position {
	if rtrn.Expression == nil {
		return EndPositionFromToken(rtrn.Token)
	}
	return rtrn.Expression.EndPos()
}

func (_ *ReturnStatement) stmtNode() {}

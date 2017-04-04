package ast

import "github.com/orktes/orlang/scanner"

type UnaryExpression struct {
	Expression
	Operator scanner.Token
	Postfix  bool
}

func (u *UnaryExpression) StartPos() Position {
	if u.Postfix {
		return u.Expression.StartPos()
	}
	return StartPositionFromToken(u.Operator)
}

func (u *UnaryExpression) EndPos() Position {
	if u.Postfix {
		return EndPositionFromToken(u.Operator)
	}
	return u.Expression.EndPos()
}

func (_ *UnaryExpression) exprNode() {}

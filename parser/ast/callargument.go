package ast

import "github.com/orktes/orlang/parser/scanner"

type CallArgument struct {
	Name       *scanner.Token
	Expression Expression
}

func (ca *CallArgument) StartPos() Position {
	if ca.Name != nil {
		return StartPositionFromToken(*ca.Name)
	}
	return ca.Expression.StartPos()
}

func (ca *CallArgument) EndPos() Position {
	return ca.Expression.EndPos()
}

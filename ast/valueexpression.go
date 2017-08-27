package ast

import "github.com/orktes/orlang/scanner"

// ValueExpression temp container for direct values
type ValueExpression struct {
	scanner.Token
}

func (v *ValueExpression) StartPos() Position {
	return StartPositionFromToken(v.Token)
}

func (v *ValueExpression) EndPos() Position {
	return EndPositionFromToken(v.Token)
}

func (_ *ValueExpression) exprNode() {
}

func (v *ValueExpression) String() string {
	return v.Text
}

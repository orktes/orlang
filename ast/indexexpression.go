package ast

import (
	"fmt"

	"github.com/orktes/orlang/scanner"
)

// IndexExpression represents array/slice indexing: arr[index]
type IndexExpression struct {
	Target       Expression
	Index        Expression
	LeftBracket  scanner.Token
	RightBracket scanner.Token
}

func (IndexExpression) exprNode() {}

func (ie *IndexExpression) StartPos() Position {
	return ie.Target.StartPos()
}

func (ie *IndexExpression) EndPos() Position {
	return EndPositionFromToken(ie.RightBracket)
}

func (ie *IndexExpression) String() string {
	return fmt.Sprintf("%s[%s]", ie.Target, ie.Index)
}

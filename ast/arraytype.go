package ast

import (
	"fmt"

	"github.com/orktes/orlang/scanner"
)

type ArrayType struct {
	LeftPracket  scanner.Token
	RightPracket scanner.Token
	Length       Expression
	Type         Type
}

func (ArrayType) typeNode() {}

func (at *ArrayType) StartPos() Position {
	return StartPositionFromToken(at.LeftPracket)
}

func (at *ArrayType) EndPos() Position {
	return at.Type.EndPos()
}

func (at *ArrayType) String() string {
	return fmt.Sprintf("[]%s", at.Type)
}

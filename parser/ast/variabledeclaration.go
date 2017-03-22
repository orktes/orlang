package ast

import "github.com/orktes/orlang/parser/scanner"

type VariableDeclaration struct {
	Name         scanner.Token
	Type         scanner.Token
	DefaultValue Expression
	Constant     bool
}

func (vd *VariableDeclaration) StartPos() Position {
	return StartPositionFromToken(vd.Name)
}

func (vd *VariableDeclaration) EndPos() Position {
	if vd.DefaultValue != nil {
		return vd.DefaultValue.EndPos()
	}
	return StartPositionFromToken(vd.Type)
}

func (_ *VariableDeclaration) stmtNode() {}

package ast

import "github.com/orktes/orlang/parser/scanner"

type VariableDeclaration struct {
	Name         *Identifier
	Type         scanner.Token
	DefaultValue Expression
	Constant     bool
}

func (vd *VariableDeclaration) StartPos() Position {
	return vd.Name.StartPos()
}

func (vd *VariableDeclaration) EndPos() Position {
	if vd.DefaultValue != nil {
		return vd.DefaultValue.EndPos()
	}
	return StartPositionFromToken(vd.Type)
}

func (_ *VariableDeclaration) stmtNode() {}

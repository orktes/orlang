package ast

import "github.com/orktes/orlang/scanner"

type FunctionSignature struct {
	Start      Position
	End        Position
	Identifier *Identifier
	Operator   *scanner.Token
	Arguments  []*Argument
	ReturnType Type
	Extern     bool
}

func (FunctionSignature) declarationNode() {}
func (FunctionSignature) typeNode()        {}

func (fs *FunctionSignature) StartPos() Position {
	return fs.Start
}

func (fs *FunctionSignature) EndPos() Position {
	return fs.End
}

func (_ *FunctionDeclaration) typeNode() {
}

type FunctionDeclaration struct {
	Signature *FunctionSignature
	Block     *Block
}

func (fd *FunctionDeclaration) StartPos() Position {
	return fd.Signature.StartPos()
}

func (fd *FunctionDeclaration) EndPos() Position {
	return fd.Block.EndPos()
}

func (*FunctionDeclaration) exprNode() {
}

func (*FunctionDeclaration) declarationNode() {}

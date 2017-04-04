package ast

type FunctionDeclaration struct {
	Start       Position
	Identifier  *Identifier
	Arguments   []*Argument
	ReturnTypes []*Argument
	Block       *Block
	Extern      bool
}

func (fd *FunctionDeclaration) StartPos() Position {
	return fd.Start
}

func (fd *FunctionDeclaration) EndPos() Position {
	if fd.Block == nil {
		return fd.Start
	}
	return fd.Block.End
}

func (_ *FunctionDeclaration) exprNode() {
}

package ast

type ForLoop struct {
	Start     Position
	Block     *Block
	Init      Node // TODO Statement interface
	Condition Expression
	After     Node // TODO Statement interface
}

func (forloop *ForLoop) StartPos() Position {
	return forloop.Start
}

func (forloop *ForLoop) EndPos() Position {
	if forloop.Block == nil {
		return forloop.Start
	}
	return forloop.Block.End
}

package ast

type If struct {
	Start     Position
	Block     *Block
	Condition Expression
	Else      *Block
}

func (i *If) StartPos() Position {
	return i.Start
}

func (i *If) EndPos() Position {
	if i.Else != nil {
		return i.Else.End
	}
	if i.Block == nil {
		return i.Start
	}
	return i.Block.End
}

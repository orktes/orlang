package ast

type IfStatement struct {
	Start     Position
	Block     *Block
	Condition Expression
	Else      *Block
}

func (i *IfStatement) StartPos() Position {
	return i.Start
}

func (i *IfStatement) EndPos() Position {
	if i.Else != nil {
		return i.Else.End
	}
	if i.Block == nil {
		return i.Start
	}
	return i.Block.End
}

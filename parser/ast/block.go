package ast

type Block struct {
	Start Position
	End   Position
	Body  []Node
}

func (blk *Block) StartPos() Position {
	return blk.Start
}

func (blk *Block) EndPos() Position {
	return blk.End
}

func (_ *Block) exprNode() {
}

func (blk *Block) AppendNode(node Node) {
	blk.Body = append(blk.Body, node)
}

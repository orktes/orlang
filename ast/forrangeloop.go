package ast

type ForRangeLoop struct {
	Start     Position
	IndexName *Identifier // nil for single-var form (for var val in ...)
	ValueName *Identifier // always present
	Iterable  Expression  // the collection being iterated
	Block     *Block
}

func (f *ForRangeLoop) StartPos() Position {
	return f.Start
}

func (f *ForRangeLoop) EndPos() Position {
	if f.Block == nil {
		return f.Start
	}
	return f.Block.End
}

func (_ *ForRangeLoop) stmtNode() {}

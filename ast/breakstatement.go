package ast

type BreakStatement struct {
	Start    Position
	BreakEnd Position
}

func (b *BreakStatement) StartPos() Position {
	return b.Start
}

func (b *BreakStatement) EndPos() Position {
	return b.BreakEnd
}

func (b *BreakStatement) String() string {
	return "break"
}

func (_ *BreakStatement) stmtNode() {}

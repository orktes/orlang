package ast

type ContinueStatement struct {
	Start       Position
	ContinueEnd Position
}

func (c *ContinueStatement) StartPos() Position {
	return c.Start
}

func (c *ContinueStatement) EndPos() Position {
	return c.ContinueEnd
}

func (c *ContinueStatement) String() string {
	return "continue"
}

func (_ *ContinueStatement) stmtNode() {}

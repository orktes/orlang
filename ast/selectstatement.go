package ast

// SelectStatement waits on multiple channel operations at once:
//
//	select {
//	    case var v = recv(ch1) { ... }
//	    case send(ch2, x) { ... }
//	    default { ... }
//	}
type SelectStatement struct {
	Start Position
	End   Position
	Cases []*SelectCase
}

func (s *SelectStatement) StartPos() Position {
	return s.Start
}

func (s *SelectStatement) EndPos() Position {
	return s.End
}

func (_ *SelectStatement) stmtNode() {}

// SelectCase is one arm of a select statement.
type SelectCase struct {
	Start     Position
	IsDefault bool
	IsSend    bool
	VarName   *Identifier // optional receive binding
	Channel   Expression  // channel operand (nil for default)
	Value     Expression  // send value (IsSend only)
	Block     *Block
}

func (c *SelectCase) StartPos() Position {
	return c.Start
}

func (c *SelectCase) EndPos() Position {
	return c.Block.EndPos()
}

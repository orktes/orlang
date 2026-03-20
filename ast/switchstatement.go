package ast

import "fmt"

type SwitchCase struct {
	Start     Position
	Value     Expression // nil for default case
	Block     *Block
	IsDefault bool
}

func (sc *SwitchCase) StartPos() Position {
	return sc.Start
}

func (sc *SwitchCase) EndPos() Position {
	if sc.Block != nil {
		return sc.Block.End
	}
	return sc.Start
}

func (sc *SwitchCase) String() string {
	if sc.IsDefault {
		return "default { ... }"
	}
	return fmt.Sprintf("case %v { ... }", sc.Value)
}

type SwitchStatement struct {
	Start      Position
	Expression Expression
	Cases      []*SwitchCase
}

func (s *SwitchStatement) StartPos() Position {
	return s.Start
}

func (s *SwitchStatement) EndPos() Position {
	if len(s.Cases) > 0 {
		return s.Cases[len(s.Cases)-1].EndPos()
	}
	return s.Start
}

func (s *SwitchStatement) String() string {
	return fmt.Sprintf("switch %v { ... }", s.Expression)
}

func (_ *SwitchStatement) stmtNode() {}

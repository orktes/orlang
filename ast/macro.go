package ast

import "github.com/orktes/orlang/scanner"

type Macro struct {
	Start    Position
	End      Position
	Name     scanner.Token
	Patterns []*MacroPattern
}

func (mcr *Macro) StartPos() Position {
	return mcr.Start
}

func (mcr *Macro) EndPos() Position {
	return mcr.End
}

type MacroTokenSet interface {
	macroTokenSet()
}

type MacroTokenSliceSet []scanner.Token

func (tss MacroTokenSliceSet) macroTokenSet() {}

type MacroRepetitionTokenSet struct {
	Sets []MacroTokenSet
}

func (mrts MacroRepetitionTokenSet) macroTokenSet() {}

type MacroMatch interface {
	macroMatch()
}

type MacroMatchArgument struct {
	Name string
	Type string
}

func (_ *MacroMatchArgument) macroMatch() {}

type MacroMatchRepetition struct {
	Pattern   []MacroMatch
	Operand   scanner.Token
	Delimiter *scanner.Token
}

func (_ *MacroMatchRepetition) macroMatch() {}

type MacroMatchToken struct {
	Token scanner.Token
}

func (_ *MacroMatchToken) macroMatch() {}

type MacroPattern struct {
	Pattern    []MacroMatch
	TokensSets []MacroTokenSet
}

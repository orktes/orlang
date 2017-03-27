package ast

import "github.com/orktes/orlang/parser/scanner"

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
	GetTokens(args []Node) []scanner.Token
}

type TokenSliceSet []scanner.Token

func (tss TokenSliceSet) GetTokens(args []Node) []scanner.Token {
	return tss
}

type MacroArgument struct {
	Name string
	Type string
}

type MacroPattern struct {
	Pattern    []*MacroArgument
	TokensSets []MacroTokenSet
}

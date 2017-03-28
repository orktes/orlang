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
	GetTokens(pattern []MacroMatch, args []interface{}) []scanner.Token
}

type TokenSliceSet []scanner.Token

func (tss TokenSliceSet) GetTokens(pattern []MacroMatch, args []interface{}) (tokens []scanner.Token) {
	tokens = make([]scanner.Token, len(tss))
	for i, token := range tss {
		if token.Type == scanner.TokenTypeMacroIdent {
			for argI, patrn := range pattern {
				if arg, ok := patrn.(*MacroMatchArgument); ok {
					if arg.Name == token.Text {
						token.Value = args[argI]
						break
					}
				}
			}
		}
		tokens[i] = token
	}
	return tokens
}

type MacroMatch interface {
	macroMatch()
}

type MacroMatchArgument struct {
	Name string
	Type string
}

func (_ *MacroMatchArgument) macroMatch() {}

type MacroMatchToken struct {
	Token scanner.Token
}

func (_ *MacroMatchToken) macroMatch() {}

type MacroPattern struct {
	Pattern    []MacroMatch
	TokensSets []MacroTokenSet
}

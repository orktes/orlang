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
	GetTokens(Pattern []*MacroArgument, args []Node) []scanner.Token
}

type TokenSliceSet []scanner.Token

func (tss TokenSliceSet) GetTokens(Pattern []*MacroArgument, args []Node) (tokens []scanner.Token) {
	tokens = make([]scanner.Token, len(tss))
	for i, token := range tss {
		if token.Type == scanner.TokenTypeMacroIdent {
			for argI, arg := range Pattern {
				if arg.Name == token.Text {
					token.Value = args[argI]
					break
				}
			}
		}
		tokens[i] = token
	}
	return tokens
}

type MacroArgument struct {
	Name string
	Type string
}

type MacroPattern struct {
	Pattern    []*MacroArgument
	TokensSets []MacroTokenSet
}

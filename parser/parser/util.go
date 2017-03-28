package parser

import "github.com/orktes/orlang/parser/scanner"

func getClosingTokenType(token scanner.Token) scanner.TokenType {
	switch token.Type {
	case scanner.TokenTypeLPAREN:
		return scanner.TokenTypeRPAREN
	case scanner.TokenTypeLBRACE:
		return scanner.TokenTypeRBRACE
	case scanner.TokenTypeLBRACK:
		return scanner.TokenTypeRBRACK
	}

	return scanner.TokenTypeUnknown
}

package parser

import (
	"fmt"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

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

func tupplePatternToTupleExpression(pattern *ast.TuplePattern) (*ast.TupleExpression, error) {
	expressions := make([]ast.Expression, len(pattern.Patterns))

	for i, pat := range pattern.Patterns {
		switch n := pat.(type) {
		case *ast.TuplePattern:
			expr, err := tupplePatternToTupleExpression(n)
			if err != nil {
				return nil, err
			}
			expressions[i] = expr
		case ast.Expression:
			expressions[i] = n
		default:
			return nil, fmt.Errorf("Can't convert %s to expression", n)
		}
	}

	return nil, nil
}

package parser

import (
	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

func (p *Parser) parseTuplePattern() (tuplePattern *ast.TuplePattern, ok bool) {
	lParen, lparenOk := p.expectToken(scanner.TokenTypeLPAREN)
	if !lparenOk {
		p.unread()
		return
	}

	patterns := []ast.Pattern{}

	for {
		if pat, patOk := p.parsePattern(); patOk {
			ok = true
			patterns = append(patterns, pat)
		} else {
			if len(patterns) > 0 {
				ok = false
				token := p.read()
				p.error(unexpected(token.StringValue(), "pattern or identifier"))
			}
			break
		}

		_, commaOK := p.expectToken(scanner.TokenTypeCOMMA)
		if !commaOK {
			p.unread()
			break
		}
	}

	rParen, rparenOk := p.expectToken(scanner.TokenTypeRPAREN)
	if !rparenOk {
		p.error(unexpectedToken(rParen, scanner.TokenTypeRPAREN))
		return
	}

	ok = true
	tuplePattern = &ast.TuplePattern{
		LeftParen:  lParen,
		RightParen: rParen,
		Patterns:   patterns,
	}

	return
}

func (p *Parser) parsePattern() (pattern ast.Pattern, ok bool) {
	_, lparenOk := p.expectToken(scanner.TokenTypeLPAREN)
	p.unread()
	if !lparenOk {
		return p.parseIdentfier()
	}

	return p.parseTuplePattern()
}

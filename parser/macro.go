package parser

import (
	"fmt"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

func (p *Parser) parseMacro() (node *ast.Macro, ok bool) {
	token, ok := p.expectToken(scanner.TokenTypeIdent)
	if !ok || token.Text != "macro" {
		ok = false
		p.unread()
		return
	}

	macroNameToken, nameOk := p.expectToken(scanner.TokenTypeIdent)
	if !nameOk {
		p.error(unexpected(macroNameToken.StringValue(), "macro name"))
		return
	}

	lbrace, lbraceOk := p.expectToken(scanner.TokenTypeLBRACE)
	if !lbraceOk {
		p.error(unexpectedToken(lbrace, scanner.TokenTypeLBRACE))
		return
	}

	patterns := []*ast.MacroPattern{}

	for {
		pattern, patternOk := p.parseMacroPattern()
		if !patternOk {
			break
		}
		patterns = append(patterns, pattern)
	}

	rbrace, rbraceOk := p.expectToken(scanner.TokenTypeRBRACE)
	if !rbraceOk {
		p.error(unexpectedToken(rbrace, scanner.TokenTypeRBRACE))
		return
	}

	node = &ast.Macro{
		Start:    ast.StartPositionFromToken(token),
		End:      ast.EndPositionFromToken(rbrace),
		Name:     macroNameToken,
		Patterns: patterns,
	}

	p.checkCommentForNode(node, false)

	return
}

func (p *Parser) parseMacroMatchRepetition() (match *ast.MacroMatchRepetition, ok bool) {
	_, ok = p.expectToken(scanner.TokenTypeDOLLAR)
	if !ok {
		p.unread()
		return
	}

	match = &ast.MacroMatchRepetition{}

	patterns, patternsOK := p.parseMacroMatchPatterns()
	if !patternsOK {
		p.error(unexpected(p.read().StringValue(), "macro pattern"))
		return
	}

	match.Pattern = patterns

read:
	delimiterOrOperand := p.readToken(false)
	switch delimiterOrOperand.Type {
	case scanner.TokenTypeADD, scanner.TokenTypeASTERISK, scanner.TokenTypeQUESTIONMARK:
		match.Operand = delimiterOrOperand
	case scanner.TokenTypeEOF:
		p.error(unexpected(delimiterOrOperand.StringValue(), "macro repetition delimeter or operand (+, * or ?)"))
		return
	default:
		// delimiter
		if match.Delimiter != nil {
			p.error("Macro repetition can only have one token as a delimiter")
			return
		}
		match.Delimiter = &delimiterOrOperand
		goto read
	}

	return
}

func (p *Parser) parseMacroMatchArgument() (macroMatchArgument *ast.MacroMatchArgument, ok bool) {
	keyToken, ok := p.expectToken(scanner.TokenTypeMacroIdent)
	if !ok {
		p.unread()
		return
	}

	colonToken, colonOk := p.expectToken(scanner.TokenTypeCOLON)
	if !colonOk {
		p.error(unexpectedToken(colonToken, scanner.TokenTypeCOLON))
		return
	}

	typeToken, typeOk := p.expectToken(scanner.TokenTypeIdent)
	if !typeOk {
		p.error(unexpected(typeToken.StringValue(), "pattern key type"))
		return
	}

	macroMatchArgument = &ast.MacroMatchArgument{
		Name: keyToken.Text,
		Type: typeToken.Text,
	}

	return
}

func (p *Parser) parseMacroMatchPatterns() (patterns []ast.MacroMatch, ok bool) {
	openingToken, ok := p.expectToken(
		scanner.TokenTypeLPAREN,
		scanner.TokenTypeLBRACE,
		scanner.TokenTypeLBRACK,
	)
	if !ok {
		p.unread()
		return
	}

	closingParenType := getClosingTokenType(openingToken)
	patterns = []ast.MacroMatch{}

	parenCount := 1

patternLoop:
	for {
		var mma ast.MacroMatch
		var pOk bool
		if mma, pOk = p.parseMacroMatchRepetition(); !pOk {
			if mma, pOk = p.parseMacroMatchArgument(); !pOk {
				token := p.read()
				switch token.Type {
				case openingToken.Type:
					parenCount++
				case closingParenType:
					parenCount--
					if parenCount == 0 {
						break patternLoop
					}
				case scanner.TokenTypeEOF:
					p.error("Expected token but got eof")
					return
				}
				mma = &ast.MacroMatchToken{Token: token}
			}
		}

		patterns = append(patterns, mma)
	}
	return
}

func (p *Parser) parseMacroTokenSets() (set []ast.MacroTokenSet, ok bool) {
	lparen, ok := p.expectToken(
		scanner.TokenTypeLPAREN,
		scanner.TokenTypeLBRACE,
		scanner.TokenTypeLBRACK,
	)
	if !ok {
		return
	}

	closingParenType := getClosingTokenType(lparen)
	tokens := []scanner.Token{}
	parenCount := 1

loop:
	for {
		t := p.readToken(false)
		switch t.Type {
		case scanner.TokenTypeDOLLAR:
			subset, mtsOk := p.parseMacroTokenSets()
			if mtsOk {
				set = append(set, ast.MacroTokenSliceSet(tokens))
				set = append(set, ast.MacroRepetitionTokenSet{Sets: subset})
				tokens = []scanner.Token{}
				asterisk, asteriskOk := p.expectToken(scanner.TokenTypeASTERISK)
				if !asteriskOk {
					p.error(unexpectedToken(asterisk, scanner.TokenTypeASTERISK))
					return
				}
				continue loop
			}
		case scanner.TokenTypeEOF:
			p.error("Expected token but got eof")
			return
		case lparen.Type:
			parenCount++
		case closingParenType:
			parenCount--
			if parenCount == 0 {
				break loop
			}
		}
		tokens = append(tokens, t)
	}

	// add rest of the tokens
	set = append(set, ast.MacroTokenSliceSet(tokens))

	return
}

func (p *Parser) parseMacroPattern() (macroPattern *ast.MacroPattern, ok bool) {
	patterns, ok := p.parseMacroMatchPatterns()
	if !ok {
		return
	}

	macroPattern = &ast.MacroPattern{
		Pattern: patterns,
	}

	colonToken, colonOk := p.expectToken(scanner.TokenTypeCOLON)
	if !colonOk {
		p.error(unexpectedToken(colonToken, scanner.TokenTypeCOLON))
		return
	}

	// Tokens
	tokenSets, tokensOK := p.parseMacroTokenSets()
	if !tokensOK {
		p.error(unexpectedToken(p.read(),
			scanner.TokenTypeLPAREN,
			scanner.TokenTypeLBRACE,
			scanner.TokenTypeLBRACK,
		))
		return
	}

	macroPattern.TokensSets = tokenSets

	return
}

func (p *Parser) parseMacroCall(nameToken scanner.Token) (ok bool) {
	macroName := nameToken.Value.(string)
	macro, macroOk := p.macros[macroName]
	endToken := nameToken

	if !macroOk || macro == nil {
		p.error(fmt.Sprintf("No macro with name %s", macroName))
		return
	}

	macroMatcher := newMacroMatcher(macro)

	lparen, lparenOk := p.expectToken(
		scanner.TokenTypeLPAREN,
		scanner.TokenTypeLBRACE,
		scanner.TokenTypeLBRACK,
	)

	if !lparenOk {
		p.unread()
		goto patternCheckLoop
	}

	{
		closingTokenType := getClosingTokenType(lparen)
		check := func(value interface{}, vOk bool) bool {
			if vOk {
				vOk = macroMatcher.feed(value)
			}
			return vOk
		}

		parenCount := 1
	loop:
		for {
			switch {
			case macroMatcher.acceptsType("block") && check(p.parseBlock()):
			case macroMatcher.acceptsType("expr") && check(p.parseExpression()):
			case macroMatcher.acceptsType("stmt") && check(p.parseStatement(false)):
			default:
				token := p.read()
				switch token.Type {
				case lparen.Type:
					parenCount++
				case closingTokenType:
					parenCount--
					if parenCount == 0 {
						break loop
					}
				case scanner.TokenTypeEOF:
					p.error("Expected token but got eof")
					return
				}

				if !check(token, true) {
					p.error(fmt.Sprintf("No rules expected token %s", token.StringValue()))
				}
			}
		}
	}

patternCheckLoop:
	matchingProcessor := macroMatcher.match()
	if matchingProcessor == nil {
		p.error("Macro call doens't match available patterns")
		return
	}

	buf, err := matchingProcessor.processor.expand(matchingProcessor.pattern.TokensSets)
	if err != nil {
		p.error(err.Error())
		return
	}

	// This is a dirty hack to get error traces to point to the macro call instead of the macro definition
	// TODO figure out the proper way to maintain both info
	for i, t := range buf {
		t.StartLine = nameToken.StartLine
		t.StartColumn = nameToken.StartColumn

		t.EndLine = endToken.EndLine
		t.EndColumn = endToken.EndColumn

		buf[i] = t
	}

	p.returnToBuffer(buf)
	ok = true

	return
}

func (p *Parser) parseMacroSubstitutionBlock() (block *ast.Block, ok bool) {
	node, ok := p.parseMacroSubstitution()
	if ok {
		if block, ok = node.(*ast.Block); !ok {
			p.unread()
		}
	}
	return
}

func (p *Parser) parseMacroSubstitutionExpression() (expr ast.Expression, ok bool) {
	node, ok := p.parseMacroSubstitution()
	if ok {
		if expr, ok = node.(ast.Expression); !ok {
			p.unread()
		}
	}
	return
}

func (p *Parser) parseMacroSubstitutionStatement() (stmt ast.Statement, ok bool) {
	node, ok := p.parseMacroSubstitution()
	if ok {
		if stmt, ok = node.(ast.Statement); !ok {
			p.unread()
		}
	}
	return
}

func (p *Parser) parseMacroSubstitution() (substitution interface{}, ok bool) {
	token, ok := p.expectToken(scanner.TokenTypeMacroIdent)
	if !ok {
		p.unread()
		return
	}

	if token.Value != nil {
		return token.Value, true
	}

	p.error(fmt.Sprintf("Could not find matching node for %s", token.Text))

	return
}

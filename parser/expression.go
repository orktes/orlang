package parser

import (
	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

var valueTypes = []scanner.TokenType{
	scanner.TokenTypeBoolean,
	scanner.TokenTypeNumber,
	scanner.TokenTypeFloat,
	scanner.TokenTypeString,
}

var unaryPrefix = []scanner.TokenType{
	scanner.TokenTypeADD,
	scanner.TokenTypeSUB,
	scanner.TokenTypeIncrement,
	scanner.TokenTypeDecrement,
	scanner.TokenTypeEXCL,
	scanner.TokenTypeTILDE,
}

var unarySuffix = []scanner.TokenType{
	scanner.TokenTypeIncrement,
	scanner.TokenTypeDecrement,
}

func (p *Parser) parseMemberExpression(target ast.Expression) (node *ast.MemberExpression, ok bool) {
	_, ok = p.expectToken(scanner.TokenTypePERIOD)
	if !ok {
		p.unread()
		return
	}

	token, propertyOk := p.expectToken(scanner.TokenTypeIdent)
	if !propertyOk {
		p.error(unexpected(token.StringValue(), "property name"))
		return
	}

	if isKeyword(token.Text) {
		p.error(reservedKeywordError(token))
		return
	}

	node = &ast.MemberExpression{
		Target:   target,
		Property: &ast.Identifier{Token: token},
	}

	return
}

func (p *Parser) parseIndexExpression(target ast.Expression) (node *ast.IndexExpression, ok bool) {
	leftBracket, ok := p.expectToken(scanner.TokenTypeLBRACK)
	if !ok {
		p.unread()
		return
	}

	index, indexOk := p.parseExpression()
	if !indexOk {
		p.error("expected index expression")
		return
	}

	rightBracket, rBracketOk := p.expectToken(scanner.TokenTypeRBRACK)
	if !rBracketOk {
		p.error(unexpectedToken(rightBracket, scanner.TokenTypeRBRACK))
		return
	}

	node = &ast.IndexExpression{
		Target:       target,
		Index:        index,
		LeftBracket:  leftBracket,
		RightBracket: rightBracket,
	}

	return
}

func (p *Parser) parseCallExpression(target ast.Expression) (node *ast.FunctionCall, ok bool) {
	_, ok = p.expectToken(scanner.TokenTypeLPAREN)
	if !ok {
		p.unread()
		return
	}

	args := make([]*ast.CallArgument, 0)
	for {
		arg, ok := p.parseCallArgument()
		if !ok {
			break
		}

		args = append(args, arg)
		_, commaOk := p.expectToken(scanner.TokenTypeCOMMA)
		if !commaOk {
			p.unread()
			break
		}
	}

	token, rParenOk := p.expectToken(scanner.TokenTypeRPAREN)
	if !rParenOk {
		p.error(unexpectedToken(token, scanner.TokenTypeRPAREN))
		return
	}

	node = &ast.FunctionCall{
		Callee:    target,
		Arguments: args,
		End:       ast.EndPositionFromToken(token),
	}

	return
}

func (p *Parser) parseCallArgument() (arg *ast.CallArgument, ok bool) {
	arg = &ast.CallArgument{}
	p.snapshot()
	tokens, namedArgument := p.expectPattern(scanner.TokenTypeIdent, scanner.TokenTypeCOLON)
	if namedArgument {
		p.commit()
		if isKeyword(tokens[0].Text) {
			p.error(reservedKeywordError(tokens[0]))
			return
		}
		arg.Name = &ast.Identifier{Token: tokens[0]}
	} else {
		p.restore()
	}

	expr, ok := p.parseExpression()
	if ok {
		arg.Expression = expr
		p.checkCommentForNode(arg, true)
	}

	return
}

func (p *Parser) parseValueExpression() (expression ast.Expression, ok bool) {
	var token scanner.Token
	if token, ok = p.expectToken(valueTypes...); !ok {
		p.unread()
		return
	}
	return &ast.ValueExpression{Token: token}, true
}

func (p *Parser) parseIdentfier() (expression *ast.Identifier, ok bool) {
	var token scanner.Token
	if token, ok = p.expectToken(scanner.TokenTypeIdent); !ok {
		p.unread()
		return
	}

	expression = &ast.Identifier{Token: token}

	if isKeyword(token.Text) {
		p.error(reservedKeywordError(token))
		return
	}

	return
}

func (p *Parser) parseUnaryExpression() (expression ast.Expression, ok bool) {
	token, prefixOk := p.expectToken(unaryPrefix...)
	if prefixOk {
		var rExpr ast.Expression
		rExpr, ok = p.parseUnaryExpression()
		if !ok {
			p.error(unexpected(p.read().StringValue(), "expression"))
			return
		}
		expression = &ast.UnaryExpression{
			Operator:   token,
			Expression: rExpr,
		}
		return
	}

	p.unread()

	check := func(expr ast.Expression, cok bool) bool {
		if cok {
			ok = cok
			expression = expr
		}

		return cok
	}

	switch {
	case check(p.parseParenExpressionOrTuple()):
	case check(p.parseFuncDecl()):
	case check(p.parseArrayExpression()):
	case check(p.parseMapExpression()):
	case check(p.parseIdentfier()):
	case check(p.parseValueExpression()):
	// case check(p.parseBlock()): this messes up for loops
	case check(p.parseMacroSubstitutionExpression()):
	default:
		return
	}

rightLoop:
	for {
		// Parse function calls, member expressions, index expressions and type casts
		switch {
		case check(p.parseAssigment(expression)):
		case check(p.parseCallExpression(expression)):
		case check(p.parseStructExpression(expression)):
		case check(p.parseMemberExpression(expression)):
		case check(p.parseIndexExpression(expression)):
		case check(p.parseTypeOperatorExpression(expression)):
		default:
			break rightLoop
		}
	}

	if ok {
		token, suffixOk := p.expectToken(unarySuffix...)
		if suffixOk {
			expression = &ast.UnaryExpression{
				Operator:   token,
				Expression: expression,
				Postfix:    true,
			}
			return
		}

		p.unread()
	}

	return
}

// binaryPrecedence assigns each binary operator a precedence level,
// following Go's conventions: multiplicative/shift/bitwise-and bind
// tightest, then additive/bitwise-or/xor, then comparisons, then &&, then
// ||. Comparison-level operators produce ComparisonExpression nodes, the
// rest BinaryExpression nodes.
var binaryPrecedence = map[scanner.TokenType]struct {
	level      int
	comparison bool
}{
	scanner.TokenTypeOr:  {1, true},
	scanner.TokenTypeAnd: {2, true},

	scanner.TokenTypeEqual:          {3, true},
	scanner.TokenTypeNotEqual:       {3, true},
	scanner.TokenTypeLess:           {3, true},
	scanner.TokenTypeGreater:        {3, true},
	scanner.TokenTypeLessOrEqual:    {3, true},
	scanner.TokenTypeGreaterOrEqual: {3, true},

	scanner.TokenTypeADD:   {4, false},
	scanner.TokenTypeSUB:   {4, false},
	scanner.TokenTypePIPE:  {4, false},
	scanner.TokenTypeCARET: {4, false},

	scanner.TokenTypeASTERISK:  {5, false},
	scanner.TokenTypeSLASH:     {5, false},
	scanner.TokenTypePERCENT:   {5, false},
	scanner.TokenTypeLSHIFT:    {5, false},
	scanner.TokenTypeRSHIFT:    {5, false},
	scanner.TokenTypeAMPERSAND: {5, false},
}

func (p *Parser) parseExpression() (expression ast.Expression, ok bool) {
	return p.parseBinaryExpression(1)
}

// parseBinaryExpression parses a left-associative chain of binary and
// comparison operators with precedence climbing: operators below minLevel
// are left for an outer call to consume.
func (p *Parser) parseBinaryExpression(minLevel int) (expression ast.Expression, ok bool) {
	expression, ok = p.parseUnaryExpression()
	if !ok {
		return
	}

	for {
		token := p.read()
		info, isOp := binaryPrecedence[token.Type]
		if !isOp || info.level < minLevel {
			p.unread()
			return
		}

		right, rightOk := p.parseBinaryExpression(info.level + 1)
		if !rightOk {
			p.error(unexpected(p.read().StringValue(), "expression"))
			return
		}

		if info.comparison {
			expression = &ast.ComparisonExpression{
				Left:     expression,
				Right:    right,
				Operator: token,
			}
		} else {
			expression = &ast.BinaryExpression{
				Left:     expression,
				Right:    right,
				Operator: token,
			}
		}
	}
}

// parseTypeOperatorExpression parses the postfix `is Type` (type assertion)
// and `as Type` (cast) operators, which bind tighter than any binary
// operator.
func (p *Parser) parseTypeOperatorExpression(left ast.Expression) (node ast.Expression, ok bool) {
	token, ok := p.expectToken(scanner.TokenTypeIs, scanner.TokenTypeAs)
	if !ok {
		p.unread()
		return
	}

	typ, typeOk := p.parseType()
	if !typeOk {
		p.error(unexpected(p.read().StringValue(), "type"))
		return
	}

	if token.Type == scanner.TokenTypeIs {
		node = &ast.TypeAssertionExpression{
			Expression: left,
			IsToken:    token,
			Type:       typ,
		}
	} else {
		node = &ast.CastExpression{
			Left:  left,
			Token: token,
			Type:  typ,
		}
	}
	ok = true
	return
}

func (p *Parser) parseExpressionList() (expressions []ast.Expression, ok bool) {

	for {
		if expr, exprOk := p.parseExpression(); exprOk {
			ok = true
			expressions = append(expressions, expr)
		} else {
			if len(expressions) > 0 {
				ok = false
				token := p.read()
				p.error(unexpected(token.StringValue(), "expression"))
			}
			break
		}

		_, commaOK := p.expectToken(scanner.TokenTypeCOMMA)
		if !commaOK {
			p.unread()
			break
		}
	}

	return
}

func (p *Parser) parseParenExpressionOrTuple() (node ast.Expression, ok bool) {
	leftToken, leftTokenOk := p.expectToken(scanner.TokenTypeLPAREN)
	if !leftTokenOk {
		p.unread()
		return
	}

	exprList, exprListOk := p.parseExpressionList()
	if !exprListOk {
		p.error(unexpected(p.read().StringValue(), "expression"))
		return
	}

	rightToken, rightTokenOk := p.expectToken(scanner.TokenTypeRPAREN)
	if !rightTokenOk {
		p.error(unexpectedToken(rightToken, scanner.TokenTypeRPAREN))
		return
	}

	if len(exprList) == 1 {
		ok = true
		node = &ast.ParenExpression{
			LeftParen:  leftToken,
			RightParen: rightToken,
			Expression: exprList[0],
		}
	} else {
		ok = true
		node = &ast.TupleExpression{
			LeftParen:   leftToken,
			RightParen:  rightToken,
			Expressions: exprList,
		}
	}

	return
}

func (p *Parser) parseArrayExpression() (node ast.Expression, ok bool) {
	typ, typOk := p.parseArrayType()
	if !typOk {
		return
	}

	lBrace, lBraceOk := p.expectToken(scanner.TokenTypeLBRACE)
	if !lBraceOk {
		p.error(unexpectedToken(lBrace, scanner.TokenTypeLBRACE))
		return
	}

	expresList, exprListOk := p.parseExpressionList()

	rBrace, rBraceOk := p.expectToken(scanner.TokenTypeRBRACE)
	if !rBraceOk {
		if !exprListOk {
			p.error(unexpected(rBrace.StringValue(), "expression list or left brace"))
			return
		}
		p.error(unexpectedToken(rBrace, scanner.TokenTypeRBRACE))
		return
	}

	ok = true
	node = &ast.ArrayExpression{
		RightBrace:  rBrace,
		LeftBrace:   lBrace,
		Expressions: expresList,
		Type:        typ.(*ast.ArrayType),
	}

	return
}

func (p *Parser) parseMapExpression() (node ast.Expression, ok bool) {
	typ, typOk := p.parseMapType()
	if !typOk {
		return
	}

	lBrace, lBraceOk := p.expectToken(scanner.TokenTypeLBRACE)
	if !lBraceOk {
		p.unread() // Unread to allow other parsing attempts
		return
	}

	var entries []*ast.MapEntry

	// Parse key-value pairs
	for {
		// Check for closing brace (empty map or end of entries)
		token := p.peek()
		if token.Type == scanner.TokenTypeRBRACE {
			break
		}

		// Parse key expression
		keyExpr, keyOk := p.parseExpression()
		if !keyOk {
			p.error(unexpected(p.read().StringValue(), "key expression"))
			return
		}

		// Expect colon
		colon, colonOk := p.expectToken(scanner.TokenTypeCOLON)
		if !colonOk {
			p.error(unexpectedToken(colon, scanner.TokenTypeCOLON))
			return
		}

		// Parse value expression
		valueExpr, valueOk := p.parseExpression()
		if !valueOk {
			p.error(unexpected(p.read().StringValue(), "value expression"))
			return
		}

		entries = append(entries, &ast.MapEntry{
			Key:   keyExpr,
			Colon: colon,
			Value: valueExpr,
		})

		// Check for comma (more entries) or closing brace
		nextToken := p.read()
		if nextToken.Type == scanner.TokenTypeRBRACE {
			p.unread()
			break
		} else if nextToken.Type != scanner.TokenTypeCOMMA {
			p.error(unexpectedToken(nextToken, scanner.TokenTypeCOMMA, scanner.TokenTypeRBRACE))
			return
		}
	}

	rBrace, rBraceOk := p.expectToken(scanner.TokenTypeRBRACE)
	if !rBraceOk {
		p.error(unexpectedToken(rBrace, scanner.TokenTypeRBRACE))
		return
	}

	ok = true
	node = &ast.MapExpression{
		Type:       typ.(*ast.MapType),
		LeftBrace:  lBrace,
		RightBrace: rBrace,
		Entries:    entries,
	}

	return
}

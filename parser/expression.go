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
		Property: token,
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
	case check(p.parseIdentfier()):
	case check(p.parseValueExpression()):
	case check(p.parseMacroSubstitutionExpression()):
	default:
		return
	}

rightLoop:
	for {
		// Parse function calls, member expressions and type casts
		switch {
		case check(p.parseAssigment(expression)):
		case check(p.parseCallExpression(expression)):
		case check(p.parseMemberExpression(expression)):
		case check(p.parseComparisonExpression(expression)):
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
			}
			return
		}

		p.unread()
	}

	return
}

func (p *Parser) parseBinaryExpression(left ast.Expression) (node *ast.BinaryExpression, ok bool) {
	token, ok := p.expectToken(
		scanner.TokenTypeADD,
		scanner.TokenTypeSUB,
		scanner.TokenTypeASTERISK,
		scanner.TokenTypeSLASH,
	)

	if !ok {
		p.unread()
		return
	}

	var right ast.Expression
	var exprOk bool
	switch token.Type {
	case scanner.TokenTypeASTERISK, scanner.TokenTypeSLASH:
		right, exprOk = p.parseUnaryExpression()
	default:
		right, exprOk = p.parseExpression()
	}

	if !exprOk {
		p.error(unexpected(p.read().StringValue(), "expression"))
		return
	}

	node = &ast.BinaryExpression{
		Operator: token,
		Left:     left,
		Right:    right,
	}

	return
}

func (p *Parser) parseExpression() (expression ast.Expression, ok bool) {
	if expression, ok = p.parseUnaryExpression(); ok {
		for {
			if binaryExpression, binaryOk := p.parseBinaryExpression(expression); binaryOk {
				expression = binaryExpression
			} else {
				break
			}
		}
	}
	return
}

func (p *Parser) parseComparisonExpression(left ast.Expression) (node ast.Expression, ok bool) {
	token, ok := p.expectToken(
		scanner.TokenTypeEqual,
		scanner.TokenTypeNotEqual,
		scanner.TokenTypeLess,
		scanner.TokenTypeGreater,
		scanner.TokenTypeLessOrEqual,
		scanner.TokenTypeGreaterOrEqual,
	)

	if !ok {
		p.unread()
		return
	}

	right, expressionOk := p.parseExpression()
	if !expressionOk {
		p.error(unexpected(p.read().StringValue(), "expression"))
		return
	}

	node = &ast.ComparisonExpression{
		Left:     left,
		Right:    right,
		Operator: token,
	}

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

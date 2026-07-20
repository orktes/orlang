package parser

import (
	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

func (p *Parser) parseStatement(block bool) (node ast.Statement, ok bool) {
	ok = true
	var check = func(n ast.Statement, ok bool) bool {
		if ok {
			node = n
		}
		return ok
	}

	switch {
	case block && check(p.parseReturnStatement()):
	case block && check(p.parseBreakStatement()):
	case block && check(p.parseContinueStatement()):
	case block && check(p.parseForLoop()):
	case block && check(p.parseIfStatement()):
	case block && check(p.parseSwitchStatement()):
	case block && check(p.parseDeferStatement()):
	case block && check(p.parseGoStatement()):
	case block && check(p.parseSelectStatement()):
	case check(p.parseMacroSubstitutionStatement()):
	case check(p.parseVarDecl()):
	default:
		ok = false
	}

	return
}

func (p *Parser) parseForLoop() (stmt ast.Statement, nodeOk bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && token.Text == keywordFor {
		// Try for-range syntax: for var val in expr { ... } or for var idx, val in expr { ... }
		p.snapshot()
		if rangeNode, rangeOk := p.tryParseForRange(token); rangeOk {
			p.commit()
			stmt = rangeNode
			nodeOk = true
			return
		}
		p.restore()

		// Regular for-loop
		nodeOk = true
		node := &ast.ForLoop{
			Start: ast.StartPositionFromToken(token),
		}
		var condition ast.Node
		var init ast.Node
		var after ast.Node

		init, statementok := p.parseStatementOrExpression(false) // Pre stuff
		token, ok := p.expectToken(scanner.TokenTypeSEMICOLON, scanner.TokenTypeLBRACE)
		if !ok {
			if statementok {
				p.error(unexpected(token.StringValue(), "; or code block"))
			} else {
				p.error(unexpected(token.StringValue(), "statement, ; or code block"))
			}

			return
		}

		if token.Type == scanner.TokenTypeLBRACE {
			p.unread()
			// TODO create isExpression to check if a node is an expression
			condition = init
			init = nil
			goto parseBlock
		}

		condition, statementok = p.parseExpression() // Condition
		if !statementok {
			p.error(unexpected(p.read().StringValue(), "expression"))
			return
		}
		token, ok = p.expectToken(scanner.TokenTypeSEMICOLON)
		if !ok {
			p.error(unexpected(token.StringValue(), ";"))
			return
		}

		after, _ = p.parseStatementOrExpression(false) // After

	parseBlock:
		block, blockOk := p.parseBlock() // Block
		if !blockOk {
			p.error(unexpected(p.read().StringValue(), "code block"))
			return
		}

		if condition != nil {
			node.Condition = condition.(ast.Expression)
		}

		node.Init = init
		node.After = after
		node.Block = block

		p.checkCommentForNode(node, false)
		stmt = node
	} else {
		p.unread()
	}
	return
}

func (p *Parser) tryParseForRange(forToken scanner.Token) (node *ast.ForRangeLoop, ok bool) {
	// Expect: var ident in expr  OR  var ident , ident in expr
	varToken := p.read()
	if varToken.Type != scanner.TokenTypeIdent || varToken.Text != keywordVar {
		return
	}

	firstName, firstOk := p.expectToken(scanner.TokenTypeIdent)
	if !firstOk {
		return
	}

	var indexName *ast.Identifier
	var valueName *ast.Identifier

	// Check for comma (two-variable form)
	commaToken := p.read()
	if commaToken.Type == scanner.TokenTypeCOMMA {
		secondName, secondOk := p.expectToken(scanner.TokenTypeIdent)
		if !secondOk {
			return
		}
		indexName = &ast.Identifier{Token: firstName}
		valueName = &ast.Identifier{Token: secondName}
	} else {
		p.unread()
		valueName = &ast.Identifier{Token: firstName}
	}

	// Expect 'in' keyword
	inToken := p.read()
	if inToken.Type != scanner.TokenTypeIdent || inToken.Text != keywordIn {
		return
	}

	// Parse iterable expression without the right-loop to avoid
	// identifier { being parsed as a struct expression.
	var iterable ast.Expression
	var iterOk bool
	if iterable, iterOk = p.parseParenExpressionOrTuple(); !iterOk {
		if iterable, iterOk = p.parseIdentfier(); !iterOk {
			if iterable, iterOk = p.parseValueExpression(); !iterOk {
				return
			}
		}
	}
	// Allow member access and index chains
	for {
		if memberExpr, memberOk := p.parseMemberExpression(iterable); memberOk {
			iterable = memberExpr
		} else if indexExpr, indexOk := p.parseIndexExpression(iterable); indexOk {
			iterable = indexExpr
		} else if callExpr, callOk := p.parseCallExpression(iterable); callOk {
			iterable = callExpr
		} else {
			break
		}
	}
	if !iterOk {
		return
	}

	// Parse block
	block, blockOk := p.parseBlock()
	if !blockOk {
		return
	}

	node = &ast.ForRangeLoop{
		Start:     ast.StartPositionFromToken(forToken),
		IndexName: indexName,
		ValueName: valueName,
		Iterable:  iterable,
		Block:     block,
	}
	ok = true
	return
}

func (p *Parser) parseIfStatement() (node *ast.IfStatement, nodeOk bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && token.Text == keywordIf {
		nodeOk = true
		node = &ast.IfStatement{
			Start: ast.StartPositionFromToken(token),
		}

		p.checkCommentForNode(node, false)

		condition, statementok := p.parseExpression() // Condition
		if !statementok {
			p.error(unexpected(p.read().StringValue(), "expression"))
			return
		}

		block, blockOk := p.parseBlock() // Block
		if !blockOk {
			p.error(unexpected(p.read().StringValue(), "code block"))
			return
		}

		node.Condition = condition
		node.Block = block

		token = p.read()
		if token.Type == scanner.TokenTypeIdent && token.Text == keywordElse {
			var elblock *ast.Block

			elif, elseOk := p.parseIfStatement()
			if elseOk {
				elblock = &ast.Block{
					Start: ast.StartPositionFromToken(token),
					End:   elif.EndPos(),
					Body:  []ast.Node{elif},
				}
			} else if elblock, elseOk = p.parseBlock(); !elseOk {
				p.error(unexpected(p.read().StringValue(), "if statement or code block"))
			}

			node.Else = elblock
		} else {
			p.unread()
		}
	} else {
		p.unread()
	}
	return
}

func (p *Parser) parseAssigment(left ast.Expression) (node ast.Expression, ok bool) {
	token, ok := p.expectToken(
		scanner.TokenTypeASSIGN,
		scanner.TokenTypeAddAssign,
		scanner.TokenTypeSubAssign,
		scanner.TokenTypeMulAssign,
		scanner.TokenTypeDivAssign,
		scanner.TokenTypeModAssign,
	)
	if !ok {
		p.unread()
		return
	}

	expression, exprOk := p.parseExpression()
	if !exprOk {
		p.error(unexpected(p.read().StringValue(), "expression"))
		return
	}

	// Desugar compound assignments: a += b  -->  a = a + b
	if token.Type != scanner.TokenTypeASSIGN {
		var opToken scanner.Token
		switch token.Type {
		case scanner.TokenTypeAddAssign:
			opToken = scanner.Token{Type: scanner.TokenTypeADD, Text: "+"}
		case scanner.TokenTypeSubAssign:
			opToken = scanner.Token{Type: scanner.TokenTypeSUB, Text: "-"}
		case scanner.TokenTypeMulAssign:
			opToken = scanner.Token{Type: scanner.TokenTypeASTERISK, Text: "*"}
		case scanner.TokenTypeDivAssign:
			opToken = scanner.Token{Type: scanner.TokenTypeSLASH, Text: "/"}
		case scanner.TokenTypeModAssign:
			opToken = scanner.Token{Type: scanner.TokenTypePERCENT, Text: "%"}
		}
		expression = &ast.BinaryExpression{
			Left:     left,
			Operator: opToken,
			Right:    expression,
		}
	}

	node = &ast.Assigment{Left: left, Right: expression}
	return
}

func (p *Parser) parseVarDecl() (node ast.Statement, ok bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && (token.Text == keywordVar || token.Text == keywordConst) {
		ok = true

		isConstant := token.Text == keywordConst
		//startPos := ast.StartPositionFromToken(token)
		// TODO set startPos based on const token
		// Single argument def
		var declaration ast.Statement
		var declOk bool
		declaration, declOk = p.parseVariableDeclaration(isConstant)
		if !declOk {
			declaration, declOk = p.parseTupleDeclaration(isConstant)
			if !declOk {
				p.error(unexpected(p.read().StringValue(), "variable or tuple declaration"))
				return
			}
		}

		node = declaration
	} else {
		p.unread()
	}
	return
}

func (p *Parser) parseVariableDeclaration(isConstant bool) (varDecl *ast.VariableDeclaration, ok bool) {
	ident, ok := p.parseIdentfier()
	if !ok {
		return
	}

	varDecl = &ast.VariableDeclaration{}
	varDecl.Constant = isConstant
	varDecl.Name = ident
	defer p.checkCommentForNode(varDecl, true)

	token, assignOk := p.expectToken(scanner.TokenTypeCOLON, scanner.TokenTypeASSIGN)
	if !assignOk {
		p.error(unexpectedToken(token, scanner.TokenTypeCOLON, scanner.TokenTypeASSIGN))
		return
	}

	if token.Type == scanner.TokenTypeCOLON {
		typ, typOk := p.parseType()
		if !typOk {
			p.error(unexpected(p.read().StringValue(), "type"))
			return
		}

		varDecl.Type = typ

		if _, defaultAssOk := p.expectToken(scanner.TokenTypeASSIGN); !defaultAssOk {
			p.unread()
			return
		}

	}

	expr, expressionOk := p.parseExpression()
	if !expressionOk {
		p.error(unexpected(p.read().StringValue(), "expression"))
		return
	}

	varDecl.DefaultValue = expr

	return
}

func (p *Parser) parseTupleDeclaration(isConstant bool) (tupleDecl *ast.TupleDeclaration, ok bool) {
	pattern, ok := p.parseTuplePattern()
	if !ok {
		return
	}

	tupleDecl = &ast.TupleDeclaration{}
	tupleDecl.Constant = isConstant
	tupleDecl.Pattern = pattern
	defer p.checkCommentForNode(tupleDecl, true)

	token, assignOk := p.expectToken(scanner.TokenTypeCOLON, scanner.TokenTypeASSIGN)
	if !assignOk {
		p.error(unexpectedToken(token, scanner.TokenTypeCOLON, scanner.TokenTypeASSIGN))
		return
	}

	if token.Type == scanner.TokenTypeCOLON {
		typ, typOk := p.parseType()
		if !typOk {
			p.error(unexpected(p.read().StringValue(), "type"))
			return
		}

		tupleDecl.Type = typ

		if _, defaultAssOk := p.expectToken(scanner.TokenTypeASSIGN); !defaultAssOk {
			p.unread()
			return
		}

	}

	expr, expressionOk := p.parseExpression()
	if !expressionOk {
		p.error(unexpected(p.read().StringValue(), "expression"))
		return
	}

	tupleDecl.DefaultValue = expr

	return
}

func (p *Parser) parseReturnStatement() (rtrnStmt *ast.ReturnStatement, ok bool) {
	token, tokOk := p.expectToken(scanner.TokenTypeIdent)

	if !tokOk {
		p.unread()
		return
	}

	if token.Text != keywordReturn {
		p.unread()
		return
	}

	expression, _ := p.parseExpression()

	rtrnStmt = &ast.ReturnStatement{
		Start:      ast.StartPositionFromToken(token),
		ReturnEnd:  ast.EndPositionFromToken(token),
		Expression: expression,
	}

	ok = true

	return
}

func (p *Parser) parseBreakStatement() (stmt *ast.BreakStatement, ok bool) {
	token := p.read()

	if token.Type != scanner.TokenTypeBreak {
		p.unread()
		return
	}

	stmt = &ast.BreakStatement{
		Start:    ast.StartPositionFromToken(token),
		BreakEnd: ast.EndPositionFromToken(token),
	}

	ok = true
	return
}

func (p *Parser) parseContinueStatement() (stmt *ast.ContinueStatement, ok bool) {
	token := p.read()

	if token.Type != scanner.TokenTypeContinue {
		p.unread()
		return
	}

	stmt = &ast.ContinueStatement{
		Start:       ast.StartPositionFromToken(token),
		ContinueEnd: ast.EndPositionFromToken(token),
	}

	ok = true
	return
}

func (p *Parser) parseDeferStatement() (stmt *ast.DeferStatement, ok bool) {
	token := p.read()

	if token.Type != scanner.TokenTypeDefer {
		p.unread()
		return
	}

	expr, exprOk := p.parseExpression()
	if !exprOk {
		p.error(unexpected(p.read().StringValue(), "function call"))
		return
	}

	call, isCall := expr.(*ast.FunctionCall)
	if !isCall {
		p.error(unexpected("expression", "function call after defer"))
		return
	}

	stmt = &ast.DeferStatement{
		Start:    ast.StartPositionFromToken(token),
		DeferEnd: ast.EndPositionFromToken(token),
		Call:     call,
	}

	ok = true
	return
}

func (p *Parser) parseGoStatement() (stmt *ast.GoStatement, ok bool) {
	token := p.read()

	if token.Type != scanner.TokenTypeGo {
		p.unread()
		return
	}

	expr, exprOk := p.parseExpression()
	if !exprOk {
		p.error(unexpected(p.read().StringValue(), "function call"))
		return
	}

	call, isCall := expr.(*ast.FunctionCall)
	if !isCall {
		p.error(unexpected("expression", "function call after go"))
		return
	}

	stmt = &ast.GoStatement{
		Start: ast.StartPositionFromToken(token),
		Call:  call,
	}

	ok = true
	return
}

func (p *Parser) parseSelectStatement() (node *ast.SelectStatement, ok bool) {
	token := p.read()
	if token.Type != scanner.TokenTypeSelect {
		p.unread()
		return
	}

	ok = true
	node = &ast.SelectStatement{
		Start: ast.StartPositionFromToken(token),
	}

	if lbrace, lbraceOk := p.expectToken(scanner.TokenTypeLBRACE); !lbraceOk {
		p.error(unexpectedToken(lbrace, scanner.TokenTypeLBRACE))
		return
	}

	for {
		token = p.read()

		if token.Type == scanner.TokenTypeRBRACE {
			node.End = ast.EndPositionFromToken(token)
			break
		}

		if token.Type == scanner.TokenTypeDefault {
			block, blockOk := p.parseBlock()
			if !blockOk {
				p.error(unexpected(p.read().StringValue(), "code block"))
				return
			}
			node.Cases = append(node.Cases, &ast.SelectCase{
				Start:     ast.StartPositionFromToken(token),
				IsDefault: true,
				Block:     block,
			})
			continue
		}

		if token.Type != scanner.TokenTypeCase {
			p.error(unexpected(token.StringValue(), "case, default or }"))
			return
		}

		selectCase := &ast.SelectCase{
			Start: ast.StartPositionFromToken(token),
		}

		// Optional receive binding: case var name = recv(ch)
		if varToken, varOk := p.expectToken(scanner.TokenTypeIdent); varOk && varToken.Text == keywordVar {
			nameToken, nameOk := p.expectToken(scanner.TokenTypeIdent)
			if !nameOk || isKeyword(nameToken.Text) {
				p.error(unexpected(nameToken.StringValue(), "variable name"))
				return
			}
			selectCase.VarName = &ast.Identifier{Token: nameToken}
			if assignToken, assignOk := p.expectToken(scanner.TokenTypeASSIGN); !assignOk {
				p.error(unexpectedToken(assignToken, scanner.TokenTypeASSIGN))
				return
			}
		} else {
			p.unread()
		}

		// The operation: recv(ch) or send(ch, value)
		opExpr, opOk := p.parseExpression()
		if !opOk {
			p.error(unexpected(p.read().StringValue(), "recv(...) or send(...)"))
			return
		}
		opCall, isCall := opExpr.(*ast.FunctionCall)
		var opName string
		if isCall {
			if ident, isIdent := opCall.Callee.(*ast.Identifier); isIdent {
				opName = ident.Text
			}
		}
		switch {
		case opName == "recv" && len(opCall.Arguments) == 1:
			selectCase.Channel = opCall.Arguments[0].Expression
		case opName == "send" && len(opCall.Arguments) == 2 && selectCase.VarName == nil:
			selectCase.IsSend = true
			selectCase.Channel = opCall.Arguments[0].Expression
			selectCase.Value = opCall.Arguments[1].Expression
		default:
			p.error(unexpected("expression", "recv(channel) or send(channel, value)"))
			return
		}

		block, blockOk := p.parseBlock()
		if !blockOk {
			p.error(unexpected(p.read().StringValue(), "code block"))
			return
		}
		selectCase.Block = block
		node.Cases = append(node.Cases, selectCase)
	}

	return
}

func (p *Parser) parseSwitchStatement() (node *ast.SwitchStatement, ok bool) {
	token := p.read()
	if token.Type != scanner.TokenTypeSwitch {
		p.unread()
		return
	}

	ok = true
	node = &ast.SwitchStatement{
		Start: ast.StartPositionFromToken(token),
	}

	// Parse the switch expression without the right-loop to avoid
	// identifier { being parsed as a struct expression.
	// We parse the primary expression (identifier, value, paren) and then
	// handle member expressions manually.
	var expr ast.Expression
	var exprOk bool
	if expr, exprOk = p.parseParenExpressionOrTuple(); !exprOk {
		if expr, exprOk = p.parseIdentfier(); !exprOk {
			if expr, exprOk = p.parseValueExpression(); !exprOk {
				p.error(unexpected(p.read().StringValue(), "expression"))
				return
			}
		}
	}
	// Allow member access chains (e.g., switch obj.field { ... })
	for {
		memberExpr, memberOk := p.parseMemberExpression(expr)
		if !memberOk {
			break
		}
		expr = memberExpr
	}
	node.Expression = expr

	// Expect opening brace for switch body
	if _, braceOk := p.expectToken(scanner.TokenTypeLBRACE); !braceOk {
		p.error(unexpected(p.read().StringValue(), "{"))
		return
	}

	// Parse cases
	for {
		token = p.read()

		if token.Type == scanner.TokenTypeRBRACE {
			break
		}

		if token.Type == scanner.TokenTypeCase {
			// Parse case value
			caseExpr, caseExprOk := p.parseExpression()
			if !caseExprOk {
				p.error(unexpected(p.read().StringValue(), "expression"))
				return
			}

			block, blockOk := p.parseBlock()
			if !blockOk {
				p.error(unexpected(p.read().StringValue(), "code block"))
				return
			}

			node.Cases = append(node.Cases, &ast.SwitchCase{
				Start: ast.StartPositionFromToken(token),
				Value: caseExpr,
				Block: block,
			})
		} else if token.Type == scanner.TokenTypeDefault {
			block, blockOk := p.parseBlock()
			if !blockOk {
				p.error(unexpected(p.read().StringValue(), "code block"))
				return
			}

			node.Cases = append(node.Cases, &ast.SwitchCase{
				Start:     ast.StartPositionFromToken(token),
				Block:     block,
				IsDefault: true,
			})
		} else {
			p.error(unexpected(token.StringValue(), "case or default"))
			return
		}
	}

	return
}

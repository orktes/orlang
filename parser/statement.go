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
	case block && check(p.parseForLoop()):
	case block && check(p.parseIfStatement()):
	case check(p.parseMacroSubstitutionStatement()):
	case check(p.parseVarDecl()):
	default:
		ok = false
	}

	return
}

func (p *Parser) parseForLoop() (node *ast.ForLoop, nodeOk bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && token.Text ==keywordFor {
		nodeOk = true
		node = &ast.ForLoop{
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

	} else {
		p.unread()
	}
	return
}

func (p *Parser) parseIfStatement() (node *ast.IfStatement, nodeOk bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && token.Text ==keywordIf {
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
		if token.Type == scanner.TokenTypeIdent && token.Text ==keywordElse {
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
	_, ok = p.expectToken(scanner.TokenTypeASSIGN)
	if !ok {
		p.unread()
		return
	}

	expression, exprOk := p.parseExpression()
	if !exprOk {
		p.error(unexpected(p.read().StringValue(), "expression"))
		return
	}

	node = &ast.Assigment{Left: left, Right: expression}
	return
}

func (p *Parser) parseVarDecl() (node ast.Statement, ok bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && (token.Text ==keywordVar || token.Text ==keywordConst) {
		ok = true

		isConstant := token.Text ==keywordConst
		startPos := ast.StartPositionFromToken(token)

		token = p.peek()
		if token.Type == scanner.TokenTypeLPAREN {
			// Multiple argument definitions
			declarations, declOk := p.parseVariableDeclarations(isConstant)
			if !declOk {
				return
			}

			node = &ast.MultiVariableDeclaration{
				Start:        startPos,
				End:          ast.EndPositionFromToken(p.lastToken()),
				Declarations: declarations,
			}
		} else {
			// Single argument def
			declaration, declOk := p.parseVariableDeclaration(isConstant)
			if !declOk {
				p.error(unexpected(p.read().StringValue(), "variable declaration"))
				return
			}

			node = declaration
		}
	} else {
		p.unread()
	}
	return
}

func (p *Parser) parseVariableDeclarations(isConstant bool) (varDecls []*ast.VariableDeclaration, ok bool) {
	if t, lparenOk := p.expectToken(scanner.TokenTypeLPAREN); !lparenOk {
		p.error(unexpectedToken(t, scanner.TokenTypeLPAREN))
		return
	}

	for {
		var foundVarDecl = false
		var varDecl *ast.VariableDeclaration
		varDecl, ok = p.parseVariableDeclaration(isConstant)
		if ok {
			foundVarDecl = true
			varDecls = append(varDecls, varDecl)
		}

		var token scanner.Token

		var foundVarDeclEnd bool
		if foundVarDecl {
			token, foundVarDeclEnd = p.expectToken(scanner.TokenTypeRPAREN, scanner.TokenTypeCOMMA)
		} else {
			token, foundVarDeclEnd = p.expectToken(scanner.TokenTypeRPAREN)
		}

		if !foundVarDeclEnd {
			if foundVarDecl {
				p.error(unexpectedToken(token, scanner.TokenTypeRPAREN, scanner.TokenTypeCOMMA))
			} else {
				p.error(unexpectedToken(token, scanner.TokenTypeIdent, scanner.TokenTypeRPAREN))
			}
			return
		}

		if token.Type == scanner.TokenTypeRPAREN {
			break
		}
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
		token, colonOk := p.parseType()
		if !colonOk {
			p.error(unexpectedToken(p.read(), scanner.TokenTypeIdent))
			return
		}

		varDecl.Type = token

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

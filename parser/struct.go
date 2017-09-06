package parser

import (
	"github.com/orktes/orlang/ast"

	"github.com/orktes/orlang/scanner"
)

func (p *Parser) parseStruct() (node *ast.Struct, ok bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && token.Text == keywordStruct {
		ok = true

		node = &ast.Struct{}

		identifier, _ := p.parseIdentfier()
		node.Name = identifier

		if leftBrace, leftBraceOk := p.expectToken(scanner.TokenTypeLBRACE); leftBraceOk {
			node.Start = ast.StartPositionFromToken(leftBrace)
		} else {
			p.error(unexpectedToken(leftBrace, scanner.TokenTypeLBRACE))
			return
		}

		for {
			if varDecl, varDeclOk := p.parseVarDecl(); varDeclOk {
				if varDecl, varDeclOk := varDecl.(*ast.VariableDeclaration); varDeclOk {
					node.Variables = append(node.Variables, varDecl)
				} else {
					p.error(unexpected("tuple declration", "variable declaration or member function"))
				}
			} else if funcDecl, funcDeclOk := p.parseFuncDecl(); funcDeclOk {
				node.Functions = append(node.Functions, funcDecl)
			} else {
				break
			}
		}

		if rightBrace, rightBraceOk := p.expectToken(scanner.TokenTypeRBRACE); rightBraceOk {
			node.End = ast.StartPositionFromToken(rightBrace)
		} else {
			p.error(unexpectedToken(rightBrace, scanner.TokenTypeRBRACE))
			return
		}

	} else {
		p.unread()
	}

	return
}

func (p *Parser) parseStructExpression(expr ast.Expression) (node *ast.StructExpression, ok bool) {
	ident, ok := expr.(*ast.Identifier)
	if !ok {
		return
	}

	_, ok = p.expectToken(scanner.TokenTypeLBRACE)
	if !ok {
		p.unread()
		return
	}

	node = &ast.StructExpression{
		Identifier: ident,
	}

	token, rBraceOk := p.expectToken(scanner.TokenTypeRBRACE)
	if !rBraceOk {
		p.error(unexpectedToken(token, scanner.TokenTypeRBRACE))
		return
	}
	return
}

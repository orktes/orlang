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

	node = &ast.StructExpression{
		Identifier: ident,
		Arguments:  args,
	}

	token, rBraceOk := p.expectToken(scanner.TokenTypeRBRACE)
	if !rBraceOk {
		p.error(unexpectedToken(token, scanner.TokenTypeRBRACE))
		return
	}
	return
}

func (p *Parser) parseInterface() (node *ast.Interface, ok bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && token.Text == keywordInterface {
		ok = true

		node = &ast.Interface{}

		identifier, _ := p.parseIdentfier()
		node.Name = identifier

		if leftBrace, leftBraceOk := p.expectToken(scanner.TokenTypeLBRACE); leftBraceOk {
			node.Start = ast.StartPositionFromToken(leftBrace)
		} else {
			p.error(unexpectedToken(leftBrace, scanner.TokenTypeLBRACE))
			return
		}

		for {
			if funcSig, funcSigOk := p.parseFuncSignature(); funcSigOk {
				node.Functions = append(node.Functions, funcSig)
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

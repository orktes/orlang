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

func (p *Parser) parseEnum() (node *ast.Enum, ok bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && token.Text == keywordEnum {
		ok = true

		node = &ast.Enum{}

		identifier, _ := p.parseIdentfier()
		node.Name = identifier

		if leftBrace, leftBraceOk := p.expectToken(scanner.TokenTypeLBRACE); leftBraceOk {
			node.Start = ast.StartPositionFromToken(leftBrace)
		} else {
			p.error(unexpectedToken(leftBrace, scanner.TokenTypeLBRACE))
			return
		}

		for {
			valueIdent, identOk := p.expectToken(scanner.TokenTypeIdent)
			if !identOk {
				p.unread()
				break
			}
			node.Values = append(node.Values, &ast.EnumValue{
				Name: &ast.Identifier{Token: valueIdent},
			})
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
	ident, identOk := expr.(*ast.Identifier)
	if !identOk {
		return
	}

	// Peek ahead to distinguish struct expression from code block
	// without consuming any tokens. This avoids the "identifier {" ambiguity
	// (e.g., "b {" in "if a && b { ... }" should not be struct expression)
	tokens := p.peekMultiple(3) // peek: {, firstArg, secondToken

	if tokens[0].Type != scanner.TokenTypeLBRACE {
		return
	}

	// Determine if this looks like a struct expression based on what follows {
	isStructExpr := false
	switch tokens[1].Type {
	case scanner.TokenTypeRBRACE:
		// Empty struct: Foo{}
		isStructExpr = true
	case scanner.TokenTypeNumber, scanner.TokenTypeFloat, scanner.TokenTypeString, scanner.TokenTypeBoolean:
		// Positional args starting with a literal: Foo{10, 20}
		isStructExpr = true
	case scanner.TokenTypeIdent:
		if !isKeyword(tokens[1].Text) {
			// Check if it's named (ident:) or positional (ident,) or single (ident})
			if tokens[2].Type == scanner.TokenTypeCOLON || tokens[2].Type == scanner.TokenTypeCOMMA || tokens[2].Type == scanner.TokenTypeRBRACE {
				isStructExpr = true
			}
		}
	case scanner.TokenTypeSUB, scanner.TokenTypeEXCL:
		// Unary prefix on a value
		isStructExpr = true
	case scanner.TokenTypeLPAREN:
		// Parenthesized expression as positional arg
		isStructExpr = true
	}

	if !isStructExpr {
		return
	}

	// Now consume the { token
	_, ok = p.expectToken(scanner.TokenTypeLBRACE)
	if !ok {
		p.unread()
		return
	}

	args := make([]*ast.CallArgument, 0)
	for {
		arg, argOk := p.parseCallArgument()
		if !argOk {
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

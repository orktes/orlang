package parser

import (
	"github.com/orktes/orlang/parser/ast"
	"github.com/orktes/orlang/parser/scanner"
)

func (p *Parser) parseFuncDecl() (node *ast.FunctionDeclaration, ok bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && (token.Text == KeywordFunction || token.Text == KeywordExtern) {
		ok = true
		node = &ast.FunctionDeclaration{}
		node.Start = ast.StartPositionFromToken(token)

		if identifier, parseIdent := p.parseIdentfier(); parseIdent {
			node.Identifier = identifier
		}

		arguments, argumentsOk := p.parseArguments()
		if !argumentsOk {
			if node.Identifier == nil {
				p.error(unexpected(p.read().StringValue(), "function name or argument list"))
			} else {
				p.error(unexpectedToken(p.read(), scanner.TokenTypeLPAREN))
			}
			return
		}

		node.Arguments = arguments

		_, returnTypeColonOk := p.expectToken(scanner.TokenTypeCOLON)
		if returnTypeColonOk {
			if returnArgs, returnArgsOk := p.parseArguments(); returnArgsOk {
				node.ReturnTypes = returnArgs
			} else if returnArg, returnArgsOk := p.parseArgument(); returnArgsOk {
				node.ReturnTypes = []*ast.Argument{returnArg}
			} else {
				p.error(unexpected(p.read().StringValue(), "function return type"))
				return
			}
		} else {
			p.unread()
		}

		if token.Text != KeywordExtern {
			blk, blockOk := p.parseBlock()
			if !blockOk {
				p.error(unexpected(p.read().StringValue(), "code block"))
				return
			}

			node.Block = blk
		}

		p.checkCommentForNode(node, false)
	} else {
		p.unread()
	}
	return
}

func (p *Parser) parseArguments() (args []*ast.Argument, ok bool) {
	if _, lparenOk := p.expectToken(scanner.TokenTypeLPAREN); !lparenOk {
		p.unread()
		return
	}

	for {
		var foundArg = false
		var arg *ast.Argument
		arg, ok = p.parseArgument()
		if ok {
			foundArg = true
			args = append(args, arg)
		}

		var token scanner.Token

		if foundArg {
			token, ok = p.expectToken(scanner.TokenTypeRPAREN, scanner.TokenTypeCOMMA)
		} else {
			token, ok = p.expectToken(scanner.TokenTypeRPAREN)
		}

		if !ok {
			if foundArg {
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

func (p *Parser) parseArgument() (arg *ast.Argument, ok bool) {
	// name : Type = DefaultValue
	identifier, ok := p.parseIdentfier()
	if !ok {
		return
	}

	arg = &ast.Argument{}
	arg.Name = identifier
	defer p.checkCommentForNode(arg, true)

	token, colonOK := p.expectToken(scanner.TokenTypeCOLON, scanner.TokenTypeASSIGN)
	if !colonOK {
		p.unread()
		return
	}

	if token.Type == scanner.TokenTypeCOLON {
		_, variadic := p.expectToken(scanner.TokenTypeEllipsis)
		arg.Variadic = variadic
		if !variadic {
			p.unread()
		}

		token, typeOk := p.parseType()
		if !typeOk {
			if !arg.Variadic {
				p.error(unexpectedToken(p.read(), scanner.TokenTypeIdent))
				return
			}
		}

		arg.Type = &token

		if _, defaultAssOk := p.expectToken(scanner.TokenTypeASSIGN); !defaultAssOk {
			p.unread()
			return
		}
	}

	expr, ok := p.parseExpression()
	if !ok {
		p.error(unexpected(p.read().StringValue(), "expression"))
		return
	}

	arg.DefaultValue = expr

	return
}

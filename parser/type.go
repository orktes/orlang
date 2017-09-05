package parser

import (
	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

func (p *Parser) parseType() (typ ast.Type, ok bool) {
	if typ, ok = p.parseTypeReference(); ok {
		return
	} else if typ, ok = p.parseTupleOrSignatureType(); ok {
		return
	} else if typ, ok = p.parseArrayType(); ok {
		return
	}
	return
}

func (p *Parser) parseTypeReference() (typ ast.Type, ok bool) {

	token, ok := p.expectToken(scanner.TokenTypeIdent)
	if !ok {
		p.unread()
		return
	}

	if isKeyword(token.Text) {
		p.error(reservedKeywordError(token))
	}

	typ = &ast.TypeReference{Token: token}

	return
}

func (p *Parser) parseTypeList() (types []ast.Type, ok bool) {

	for {
		if typ, typOk := p.parseType(); typOk {
			ok = true
			types = append(types, typ)
		} else {
			if len(types) > 0 {
				ok = false
				token := p.read()
				p.error(unexpected(token.StringValue(), "type"))
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

func (p *Parser) parseTupleOrSignatureType() (node ast.Type, ok bool) {
	leftToken, leftTokenOk := p.expectToken(scanner.TokenTypeLPAREN)
	if !leftTokenOk {
		p.unread()
		return
	}

	typeList, typeListOk := p.parseTypeList()
	if !typeListOk {
		p.error(unexpected(p.read().StringValue(), "type"))
		return
	}

	rightToken, rightTokenOk := p.expectToken(scanner.TokenTypeRPAREN)
	if !rightTokenOk {
		p.error(unexpectedToken(rightToken, scanner.TokenTypeRPAREN))
		return
	}

	ok = true
	node = &ast.TupleType{
		LeftParen:  leftToken,
		RightParen: rightToken,
		Types:      typeList,
	}

	_, returnTypeColonOk := p.expectToken(scanner.TokenTypeArrow)
	if returnTypeColonOk {
		// It is actually a signature type
		if returnType, returnTypeOk := p.parseType(); returnTypeOk {
			signature := &ast.FunctionSignature{}
			signature.Start = node.StartPos()
			signature.End = node.EndPos()
			signature.ReturnType = returnType
			args := make([]*ast.Argument, len(typeList))
			for i, typ := range typeList {
				args[i] = &ast.Argument{
					Type: typ,
				}
			}
			signature.Arguments = args
			node = signature
		} else {
			p.error(unexpected(p.read().StringValue(), "function return type"))
			return
		}
	} else {
		p.unread()
	}

	return
}

func (p *Parser) parseArrayType() (node ast.Type, ok bool) {
	var lengthExpression ast.Expression
	var lengthExpressionOk bool
	leftToken, leftTokenOk := p.expectToken(scanner.TokenTypeLBRACK)
	if !leftTokenOk {
		p.unread()
		return
	}

	ok = true

	rightToken, rightTokenOk := p.expectToken(scanner.TokenTypeRBRACK)
	if rightTokenOk {
		goto parseType
	} else {
		p.unread()
	}

	lengthExpression, lengthExpressionOk = p.parseExpression()
	if !lengthExpressionOk {
		p.error(unexpected(p.read().StringValue(), "length expression"))
		return
	}

	if rightToken, rightTokenOk = p.expectToken(scanner.TokenTypeRBRACK); !rightTokenOk {
		p.error(unexpectedToken(rightToken, scanner.TokenTypeRBRACK))
		return
	}

parseType:
	typ, typOk := p.parseType()

	if !typOk {
		p.error(unexpected(p.read().StringValue(), "array type"))
		return
	}

	ok = true
	node = &ast.ArrayType{
		LeftPracket:  leftToken,
		RightPracket: rightToken,
		Length:       lengthExpression,
		Type:         typ,
	}

	return
}

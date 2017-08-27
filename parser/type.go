package parser

import (
	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

func (p *Parser) parseType() (typ ast.Type, ok bool) {
	if typ, ok = p.parseTypeReference(); ok {
		return
	} else if typ, ok = p.parseTuppleType(); ok {
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

func (p *Parser) parseTuppleType() (node ast.Type, ok bool) {
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
	node = &ast.TuppleType{
		LeftParen:  leftToken,
		RightParen: rightToken,
		Types:      typeList,
	}

	return
}

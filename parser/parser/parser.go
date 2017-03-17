package parser

import (
	"fmt"
	"io"

	"github.com/orktes/orlang/parser/ast"

	"github.com/orktes/orlang/parser/scanner"
)

var valueTypes = []scanner.TokenType{
	scanner.TokenTypeIdent,
	scanner.TokenTypeBoolean,
	scanner.TokenTypeNumber,
	scanner.TokenTypeFloat,
	scanner.TokenTypeString,
}

type Parser struct {
	s           *scanner.Scanner
	tokenBuffer []scanner.Token
	lastTokens  []scanner.Token
	scanChan    <-chan scanner.Token
	parserError string
	Error       func(Pos, msg string)
}

func NewParser(s *scanner.Scanner) *Parser {
	return &Parser{s: s, scanChan: s.ScanChannel()}
}

func Parse(reader io.Reader) (file *ast.File, err error) {
	return NewParser(scanner.NewScanner(reader)).Parse()
}

func (p *Parser) Parse() (file *ast.File, err error) {
	file = &ast.File{}
	p.s.Error = p.error

loop:
	for {
		var node ast.Node
		var check = func(n ast.Node, ok bool) bool {
			if ok {
				node = n
			}
			return ok
		}
		switch {
		case check(p.parseFuncDecl()):
		case check(p.parseVarDecl()):
		case check(p.parseConstDecl()):
		case check(p.parseImportDecl()):
		case p.eof():
			break loop
		default:
			token := p.read()
			err = PosError{Position: ast.StartPositionFromToken(token), Message: unexpectedToken(token)}
			break
		}

		if node != nil {
			file.AppendNode(node)
		}

		if p.parserError != "" {
			token := p.lastToken()
			err = PosError{Position: ast.StartPositionFromToken(token), Message: p.parserError}
			break
		}
	}

	return
}

func (p *Parser) eof() (ok bool) {
	if _, ok = p.expectToken(scanner.TokenTypeEOF); !ok {
		p.unread()
	}
	return
}

func (p *Parser) parseFuncDecl() (node *ast.FunctionDeclaration, ok bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && token.Text == "fn" {
		ok = true
		node = &ast.FunctionDeclaration{}
		node.Start = ast.StartPositionFromToken(token)

		funcNameToken := p.read()
		if funcNameToken.Type != scanner.TokenTypeIdent {
			p.error(unexpectedToken(funcNameToken, scanner.TokenTypeIdent))
			return
		}

		node.Name = funcNameToken

		arguments, argumentsOk := p.parseArguments()
		if !argumentsOk {
			p.error(unexpectedToken(p.read(), scanner.TokenTypeLPAREN))
			return
		}

		node.Arguments = arguments

		blk, blockOk := p.parseBlock()
		if !blockOk {
			p.error(unexpectedToken(p.read(), scanner.TokenTypeLBRACE))
			return
		}

		node.Block = blk
	} else {
		p.unread()
	}
	return
}

func (p *Parser) parseArguments() (args []ast.Argument, ok bool) {
	if t, lparenOk := p.expectToken(scanner.TokenTypeLPAREN); !lparenOk {
		p.error(unexpectedToken(t, scanner.TokenTypeLPAREN))
		return
	}

	for {
		var foundArg = false
		var arg ast.Argument
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

func (p *Parser) parseArgument() (arg ast.Argument, ok bool) {
	var token scanner.Token
	// name : Type = DefaultValue
	token, ok = p.expectToken(scanner.TokenTypeIdent)
	if !ok {
		p.unread()
		return
	}

	arg.Name = token

	token, ok = p.expectToken(scanner.TokenTypeCOLON)
	if !ok {
		p.error(unexpectedToken(token, scanner.TokenTypeCOLON))
		return
	}

	token, ok = p.expectToken(scanner.TokenTypeIdent)
	if !ok {
		p.error(unexpectedToken(token, scanner.TokenTypeIdent))
		return
	}

	arg.Type = token

	if _, defaultAssOk := p.expectToken(scanner.TokenTypeASSIGN); !defaultAssOk {
		p.unread()
		return
	}

	if token, ok = p.expectToken(valueTypes...); !ok {
		p.error(unexpectedToken(token, valueTypes...))
		return
	}

	arg.DefaultValue = &token

	return
}

func (p *Parser) parseBlock() (node *ast.Block, ok bool) {
	if _, lok := p.expectToken(scanner.TokenTypeLBRACE); !lok {
		p.unread()
		return
	}

	node = &ast.Block{}

	if _, rok := p.expectToken(scanner.TokenTypeRBRACE); !rok {
		return
	}

	ok = true

	return
}

func (p *Parser) parseVarDecl() (node ast.Node, ok bool) {
	return
}

func (p *Parser) parseConstDecl() (node ast.Node, ok bool) {
	return
}

func (p *Parser) parseImportDecl() (node ast.Node, ok bool) {
	return
}

func (p *Parser) isWhitespace() bool {
	token := p.read()
	if token.Type != scanner.TokenTypeWhitespace {
		p.unread()
		return false
	}

	return true
}

func (p *Parser) expectPattern(tokenTypes ...scanner.TokenType) (tokens []scanner.Token, ok bool) {
	for _, tokenType := range tokenTypes {
		token := p.read()
		if token.Type != tokenType {
			ok = false
			break
		}

		tokens = append(tokens, token)
	}

	return
}

func (p *Parser) expectToken(tokenTypes ...scanner.TokenType) (token scanner.Token, ok bool) {
	token = p.read()
	for _, tokenType := range tokenTypes {
		if token.Type == tokenType {
			ok = true
		}
	}
	return
}

func (p *Parser) read() (token scanner.Token) {
	if len(p.tokenBuffer) > 0 {
		token = p.tokenBuffer[0]
		p.tokenBuffer = p.tokenBuffer[1:]
	} else {
		for tok := range p.scanChan {
			if tok.Type != scanner.TokenTypeWhitespace {
				token = tok
				break
			}
		}
	}

	p.lastTokens = []scanner.Token{token}

	return
}

func (p *Parser) unread() {
	p.tokenBuffer = append(p.tokenBuffer, p.lastTokens...)
	p.lastTokens = []scanner.Token{}
}

func (p *Parser) lastToken() (token scanner.Token) {
	if len(p.lastTokens) > 0 {
		return p.lastTokens[len(p.lastTokens)-1]
	}

	return p.peek()
}

func (p *Parser) skip() {
	p.skipMultiple(1)
}

func (p *Parser) skipMultiple(amount int) {
	for i := 0; i < amount; i++ {
		p.read()
		p.lastTokens = []scanner.Token{}
	}
}

func (p *Parser) peek() scanner.Token {
	return p.peekMultiple(1)[0]
}

func (p *Parser) peekMultiple(amount int) (tokens []scanner.Token) {
	tokens = make([]scanner.Token, amount)
	for i := 0; i < amount; i++ {
		tokens[i] = p.read()
	}

	p.tokenBuffer = append(p.tokenBuffer, tokens...)
	p.lastTokens = []scanner.Token{}
	return
}

func (p *Parser) error(err string) {
	if p.parserError == "" {
		p.parserError = err
	}

}

func (p *Parser) errorf(err string, args ...interface{}) {
	p.error(fmt.Sprintf(err, args...))
}

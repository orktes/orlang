package parser

import (
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
			if node.(*ast.FunctionDeclaration).Name.Text == "" {
				p.error("Root level functions can't be anonymous")
			}
		case check(p.parseVarDecl()):
		case check(p.parseImportDecl()):
		case p.eof():
			break loop
		default:
			token := p.read()
			err = PosError{Position: ast.StartPositionFromToken(token), Message: unexpectedToken(token)}
			break loop
		}

		if node != nil {
			file.AppendNode(node)
		}

		if p.parserError != "" && err == nil {
			token := p.lastToken()
			err = PosError{Position: ast.StartPositionFromToken(token), Message: p.parserError}
			break loop
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

		funcNameTokenOrLeftParen := p.read()
		if funcNameTokenOrLeftParen.Type == scanner.TokenTypeIdent {
			node.Name = funcNameTokenOrLeftParen
		} else if funcNameTokenOrLeftParen.Type == scanner.TokenTypeLPAREN {
			p.unread()
		} else {
			p.error(unexpectedToken(funcNameTokenOrLeftParen, scanner.TokenTypeIdent, scanner.TokenTypeLPAREN))
			return
		}

		arguments, argumentsOk := p.parseArguments()
		if !argumentsOk {
			p.error(unexpectedToken(p.read(), scanner.TokenTypeLPAREN))
			return
		}

		node.Arguments = arguments

		blk, blockOk := p.parseBlock()
		if !blockOk {
			p.error(unexpected(p.read().Type.String(), "code block"))
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

	token, ok = p.expectToken(scanner.TokenTypeCOLON, scanner.TokenTypeASSIGN)
	if !ok {
		p.error(unexpectedToken(token, scanner.TokenTypeCOLON, scanner.TokenTypeASSIGN))
		return
	}

	if token.Type == scanner.TokenTypeCOLON {
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
	}

	expr, ok := p.parseExpression()
	if !ok {
		p.error(unexpected(p.read().Type.String(), "expression"))
		return
	}

	arg.DefaultValue = expr

	return
}

func (p *Parser) parseBlock() (node *ast.Block, ok bool) {
	if _, lok := p.expectToken(scanner.TokenTypeLBRACE); !lok {
		p.unread()
		return
	}

	node = &ast.Block{}

loop:
	for {
		var blockNode ast.Node
		var check = func(n ast.Node, ok bool) bool {
			if ok {
				blockNode = n
			}
			return ok
		}
		switch {
		case check(p.parseStatement(true)):
		default:
			if _, rok := p.expectToken(scanner.TokenTypeRBRACE); !rok {
				p.unread()
				return
			}

			break loop
		}

		if blockNode != nil {
			node.AppendNode(blockNode)
		}
	}

	ok = true

	return
}

func (p *Parser) parseStatement(block bool) (node ast.Node, ok bool) {
	ok = true
	var check = func(n ast.Node, ok bool) bool {
		if ok {
			node = n
		}
		return ok
	}

	switch {
	case block && check(p.parseForLoop()):
	case block && check(p.parseIfStatement()):
	case check(p.parseVarDecl()):
		if block {
			if token, tok := p.expectToken(scanner.TokenTypeSEMICOLON); !tok {
				p.error(unexpectedToken(token, scanner.TokenTypeSEMICOLON))
				return
			}
		}
	case check(p.parseAssigment()):
	case check(p.parseExpression()):
	default:
		ok = false
	}

	return
}

func (p *Parser) parseForLoop() (node ast.Node, nodeOk bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && token.Text == "for" {
		nodeOk = true
		_, statementok := p.parseStatement(false) // Pre stuff
		token, ok := p.expectToken(scanner.TokenTypeSEMICOLON, scanner.TokenTypeLBRACE)
		if !ok {
			if statementok {
				p.error(unexpected(token.Type.String(), "; or code block"))
			} else {
				p.error(unexpected(token.Type.String(), "statement, ; or code block"))
			}

			return
		}

		if token.Type == scanner.TokenTypeLBRACE {
			p.unread()
			// TODO this means that the first one is also the condition
			goto parseBlock
		}

		_, statementok = p.parseStatement(false) // Condition
		if !statementok {
			p.error(unexpected(p.read().Type.String(), "statement"))
			return
		}
		token, ok = p.expectToken(scanner.TokenTypeSEMICOLON)
		if !ok {
			p.error(unexpected(token.Type.String(), ";"))
			return
		}

		p.parseStatement(false) // After

	parseBlock:
		_, ok = p.parseBlock() // Block
		if !ok {
			p.error(unexpected(p.read().Type.String(), "code block"))
			return
		}

	} else {
		p.unread()
	}
	return
}

func (p *Parser) parseIfStatement() (node ast.Node, ok bool) {
	return
}

func (p *Parser) parseAssigment() (node ast.Node, ok bool) {
	return
}

func (p *Parser) parseCallExpression() (node ast.Node, ok bool) {
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

func (p *Parser) parseExpression() (expression ast.Expression, ok bool) {
	check := func(expr ast.Expression, cok bool) bool {
		if cok {
			ok = cok
			expression = expr
		}

		return cok
	}

	switch {
	case check(p.parseCallExpression()):
	case check(p.parseFuncDecl()):
	case check(p.parseValueExpression()):
	}

	return
}

func (p *Parser) parseVarDecl() (node ast.Node, ok bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && (token.Text == "var" || token.Text == "const") {
		ok = true

		isConstant := token.Text == "const"
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
				p.error(unexpected(p.read().Type.String(), "variable declaration"))
				return
			}

			node = &declaration
		}
	} else {
		p.unread()
	}
	return
}

func (p *Parser) parseVariableDeclarations(isConstant bool) (varDecls []ast.VariableDeclaration, ok bool) {
	if t, lparenOk := p.expectToken(scanner.TokenTypeLPAREN); !lparenOk {
		p.error(unexpectedToken(t, scanner.TokenTypeLPAREN))
		return
	}

	for {
		var foundVarDecl = false
		var varDecl ast.VariableDeclaration
		varDecl, ok = p.parseVariableDeclaration(isConstant)
		if ok {
			foundVarDecl = true
			varDecls = append(varDecls, varDecl)
		}

		var token scanner.Token

		if foundVarDecl {
			token, ok = p.expectToken(scanner.TokenTypeRPAREN, scanner.TokenTypeCOMMA)
		} else {
			token, ok = p.expectToken(scanner.TokenTypeRPAREN)
		}

		if !ok {
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

func (p *Parser) parseVariableDeclaration(isConstant bool) (varDecl ast.VariableDeclaration, ok bool) {
	var token scanner.Token
	// name : Type = DefaultValue
	token, ok = p.expectToken(scanner.TokenTypeIdent)
	if !ok {
		p.unread()
		return
	}

	varDecl.Constant = isConstant
	varDecl.Name = token

	token, ok = p.expectToken(scanner.TokenTypeCOLON, scanner.TokenTypeASSIGN)
	if !ok {
		p.error(unexpectedToken(token, scanner.TokenTypeCOLON, scanner.TokenTypeASSIGN))
		return
	}

	if token.Type == scanner.TokenTypeCOLON {
		token, ok = p.expectToken(scanner.TokenTypeIdent)
		if !ok {
			p.error(unexpectedToken(token, scanner.TokenTypeIdent))
			return
		}

		varDecl.Type = token

		if _, defaultAssOk := p.expectToken(scanner.TokenTypeASSIGN); !defaultAssOk {
			p.unread()
			return
		}
	}

	expr, ok := p.parseExpression()
	if !ok {
		p.error(unexpected(p.read().Type.String(), "expression"))
		return
	}

	varDecl.DefaultValue = expr

	return
}

func (p *Parser) parseImportDecl() (node ast.Node, ok bool) {
	return
}

func (p *Parser) expectPattern(tokenTypes ...scanner.TokenType) (tokens []scanner.Token, ok bool) {
	ok = true
	for _, tokenType := range tokenTypes {
		token := p.read()
		tokens = append(tokens, token)
		if token.Type != tokenType {
			ok = false
			break
		}
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
	p.returnToBuffer(p.lastTokens)
}

func (p *Parser) returnToBuffer(tokens []scanner.Token) {
	p.tokenBuffer = append(p.tokenBuffer, tokens...)
	p.lastTokens = []scanner.Token{}
}

func (p *Parser) lastToken() (token scanner.Token) {
	if len(p.lastTokens) > 0 {
		return p.lastTokens[len(p.lastTokens)-1]
	}

	// This should not happen
	panic("No token in buffer")
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

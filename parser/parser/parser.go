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

var unaryPrefix = []scanner.TokenType{
	scanner.TokenTypeADD,
	scanner.TokenTypeSUB,
	scanner.TokenTypeIncrement,
	scanner.TokenTypeDecrement,
	scanner.TokenTypeEXCL,
}

var unarySuffix = []scanner.TokenType{
	scanner.TokenTypeIncrement,
	scanner.TokenTypeDecrement,
}

type Parser struct {
	s                *scanner.Scanner
	tokenBuffer      []scanner.Token
	lastTokens       []scanner.Token
	parserError      string
	errorToken       scanner.Token
	Error            func(tokenIndx int, pos ast.Position, msg string)
	ContinueOnErrors bool
	snapshots        [][]scanner.Token
	readTokens       int
	// comments attaching
	nodeComments          map[ast.Node][]ast.Comment
	comments              []ast.Comment
	commentAfterNodeCheck ast.Node
	// macros
	macros map[string]*ast.Macro
}

// NewParser return new Parser for a given scanner
func NewParser(s *scanner.Scanner) *Parser {
	return &Parser{
		s:            s,
		nodeComments: map[ast.Node][]ast.Comment{},
		macros:       map[string]*ast.Macro{},
	}
}

// Parse source code from io.Reader
func Parse(reader io.Reader) (file *ast.File, err error) {
	return NewParser(scanner.NewScanner(reader)).Parse()
}

// Parse source code but consuming io.Parser
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
		case check(p.parseExportDecl()):
		case p.eof():
			break loop
		case check(p.parseMacro()):
			if node != nil {
				macro, isMacro := node.(*ast.Macro)
				if isMacro && macro != nil {
					p.macros[macro.Name.Text] = macro
				}
			}
		default:
			token := p.read()
			p.error(unexpectedToken(token))
		}

		if node != nil {
			file.AppendNode(node)
		}

		if p.parserError != "" {
			token := p.errorToken
			posError := &PosError{Position: ast.StartPositionFromToken(token), Message: p.parserError}
			p.parserError = ""
			if !p.ContinueOnErrors {
				err = posError
				break loop
			}
		}
	}

	file.Comments = p.comments
	file.NodeComments = p.nodeComments
	file.Macros = p.macros

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
	if token.Type == scanner.TokenTypeIdent && (token.Text == KeywordFunction || token.Text == KeywordExtern) {
		ok = true
		node = &ast.FunctionDeclaration{}
		node.Start = ast.StartPositionFromToken(token)

		funcNameTokenOrLeftParen := p.read()
		if funcNameTokenOrLeftParen.Type == scanner.TokenTypeIdent {
			if isKeyword(funcNameTokenOrLeftParen.Text) {
				p.error(reservedKeywordError(funcNameTokenOrLeftParen))
				return
			}
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
	var token scanner.Token
	// name : Type = DefaultValue
	token, ok = p.expectToken(scanner.TokenTypeIdent)
	if !ok {
		p.unread()
		return
	}

	if isKeyword(token.Text) {
		p.error(reservedKeywordError(token))
		return
	}

	arg = &ast.Argument{}
	arg.Name = token
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

func (p *Parser) parseStatementOrExpression(block bool) (node ast.Node, ok bool) {
	if node, ok = p.parseStatement(block); !ok {
		node, ok = p.parseExpression()
	}
	return
}

func (p *Parser) parseBlock() (node *ast.Block, ok bool) {
	if node, ok = p.parseMacroSubstitutionBlock(); ok {
		return
	}

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
		case check(p.parseStatementOrExpression(true)):
		default:
			if _, tok := p.expectToken(scanner.TokenTypeRBRACE); !tok {
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

func (p *Parser) parseImportDecl() (node ast.Node, ok bool) {
	return
}

func (p *Parser) parseExportDecl() (node ast.Node, ok bool) {
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

func (p *Parser) readToken(expandMacros bool) (token scanner.Token) {
readToken:
	if len(p.tokenBuffer) > 0 {
		token = p.tokenBuffer[0]
		p.tokenBuffer = p.tokenBuffer[1:]
	} else {
		for {
			tok := p.s.Scan()
			// TODO convert NEWLINES to semicolons on some scenarios
			if tok.Type == scanner.TokenTypeComment {
				p.processComment(tok)
			} else if tok.Type != scanner.TokenTypeWhitespace {
				token = tok
				p.readTokens++
				break
			}
		}
	}

	p.lastTokens = []scanner.Token{token}

	if len(p.snapshots) > 0 {
		p.snapshots[len(p.snapshots)-1] = append(p.snapshots[len(p.snapshots)-1], token)
	}

	if expandMacros && token.Type == scanner.TokenTypeMacroCallIdent {
		if p.parseMacroCall(token) {
			goto readToken
		} else {
			// TODO throw error or something here
		}
	}

	return
}

func (p *Parser) read() (token scanner.Token) {
	return p.readToken(true)
}

func (p *Parser) unread() {
	if len(p.snapshots) > 0 {
		snapshot := p.snapshots[len(p.snapshots)-1]
		p.snapshots[len(p.snapshots)-1] = snapshot[:len(snapshot)-1]
	}
	p.returnToBuffer(p.lastTokens)
}

func (p *Parser) returnToBuffer(tokens []scanner.Token) {
	buffer := make([]scanner.Token, 0, len(tokens)+len(p.tokenBuffer))
	buffer = append(buffer, tokens...)
	buffer = append(buffer, p.tokenBuffer...)
	p.tokenBuffer = buffer
	p.lastTokens = []scanner.Token{}
}

func (p *Parser) lastToken() (token scanner.Token) {
	if len(p.lastTokens) > 0 {
		return p.lastTokens[len(p.lastTokens)-1]
	}

	// This should not happen
	// TODO figure out why we sometimes endup here
	return p.peek()
}

func (p *Parser) skip() {
	p.skipMultiple(1)
}

func (p *Parser) skipMultiple(amount int) {
	for i := 0; i < amount; i++ {
		p.read()
	}
	p.lastTokens = []scanner.Token{}
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

func (p *Parser) snapshot() {
	p.snapshots = append(p.snapshots, []scanner.Token{})
}

func (p *Parser) restore() {
	if len(p.snapshots) > 0 {
		p.returnToBuffer(p.snapshots[len(p.snapshots)-1])
		p.commit()
	}
}

func (p *Parser) commit() {
	if len(p.snapshots) > 0 {
		p.snapshots = p.snapshots[:len(p.snapshots)-1]
	}
}

func (p *Parser) error(err string) {
	if p.parserError == "" {
		p.parserError = err
		p.errorToken = p.lastToken()
	}
	if p.Error != nil {
		p.Error(p.readTokens-len(p.tokenBuffer), ast.StartPositionFromToken(p.lastToken()), err)
	}
}

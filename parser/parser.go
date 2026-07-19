package parser

import (
	"io"

	"github.com/orktes/orlang/ast"

	"github.com/orktes/orlang/scanner"
)

type Parser struct {
	s                scanner.ScannerInterface
	tokenBuffer      []scanner.Token
	lastTokens       []scanner.Token
	parserError      string
	errorToken       scanner.Token
	Error            func(tokenIndx int, pos ast.Position, endPos ast.Position, msg string)
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
func NewParser(s scanner.ScannerInterface) *Parser {
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

// Parse source code
func (p *Parser) Parse() (file *ast.File, err error) {
	file = &ast.File{}
	p.s.SetErrorCallback(p.error)

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
		case check(p.parseStruct()):
		case check(p.parseEnum()):
		case check(p.parseInterface()):
		case check(p.parseImportDecl()):
		case check(p.parseExportDecl()):
		case check(p.parseIncludeDecl()):
		case check(p.parseLinkDecl()):
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

func (p *Parser) parseStatementOrExpression(block bool) (node ast.Node, ok bool) {
	if node, ok = p.parseStatement(block); !ok {
		node, ok = p.parseExpression()
	}
	return
}

func (p *Parser) parseImportDecl() (node ast.Node, ok bool) {
	if _, ok = p.expectToken(scanner.TokenTypeImport); !ok {
		p.unread()
		return
	}

	importStmt := &ast.ImportStatement{}

	if _, ok = p.expectToken(scanner.TokenTypeLBRACE); !ok {
		p.unread()
		p.error(unexpectedToken(p.read(), scanner.TokenTypeLBRACE))
		return
	}

	for {
		var ident *ast.Identifier
		if ident, ok = p.parseIdentfier(); !ok {
			p.error(unexpectedToken(p.read(), scanner.TokenTypeIdent))
			return
		}

		item := &ast.ImportItem{Name: ident}

		// Check for "as alias"
		tok := p.read()
		if tok.Type == scanner.TokenTypeAs {
			var alias *ast.Identifier
			if alias, ok = p.parseIdentfier(); !ok {
				p.error(unexpectedToken(p.read(), scanner.TokenTypeIdent))
				return
			}
			item.Alias = alias
			// Read next token after alias
			tok = p.read()
		}

		importStmt.Items = append(importStmt.Items, item)

		if tok.Type == scanner.TokenTypeCOMMA {
			// Continue to next identifier
			continue
		} else if tok.Type == scanner.TokenTypeRBRACE {
			// End of imports list
			break
		} else {
			p.error(unexpectedToken(tok, scanner.TokenTypeCOMMA, scanner.TokenTypeRBRACE))
			return
		}
	}

	if _, ok = p.expectToken(scanner.TokenTypeFrom); !ok {
		p.unread()
		p.error(unexpectedToken(p.read(), scanner.TokenTypeFrom))
		return
	}

	var path ast.Node
	if path, ok = p.parseValueExpression(); !ok {
		p.error(unexpectedToken(p.read(), scanner.TokenTypeString))
		return
	}
	importStmt.Path = path.(*ast.ValueExpression)

	return importStmt, true
}

func (p *Parser) parseExportDecl() (node ast.Node, ok bool) {
	if _, ok = p.expectToken(scanner.TokenTypeExport); !ok {
		p.unread()
		return
	}

	var decl ast.Node
	if decl, ok = p.parseFuncDecl(); ok {
	} else if decl, ok = p.parseVarDecl(); ok {
	} else if decl, ok = p.parseStruct(); ok {
	} else if decl, ok = p.parseInterface(); ok {
	} else {
		p.error("expected declaration after export")
		return
	}

	return &ast.ExportStatement{Declaration: decl}, true
}

func (p *Parser) parseIncludeDecl() (node ast.Node, ok bool) {
	if _, ok = p.expectToken(scanner.TokenTypeInclude); !ok {
		p.unread()
		return
	}

	var path ast.Node
	if path, ok = p.parseValueExpression(); !ok {
		p.error(unexpectedToken(p.read(), scanner.TokenTypeString))
		return
	}

	return &ast.IncludeStatement{Path: path.(*ast.ValueExpression)}, true
}

func (p *Parser) parseLinkDecl() (node ast.Node, ok bool) {
	var linkToken scanner.Token
	if linkToken, ok = p.expectToken(scanner.TokenTypeLink); !ok {
		p.unread()
		return
	}

	// Check for optional sub-keyword: pkg or lib
	kind := ast.LinkKindFile
	token := p.read()
	if token.Type == scanner.TokenTypeIdent {
		switch token.Text {
		case "pkg":
			kind = ast.LinkKindPkg
		case "lib":
			kind = ast.LinkKindLib
		default:
			p.error(unexpectedToken(token, scanner.TokenTypeString))
			return
		}
	} else {
		// Not a sub-keyword, put it back — must be a string literal
		p.unread()
	}

	var path ast.Node
	if path, ok = p.parseValueExpression(); !ok {
		p.error(unexpectedToken(p.read(), scanner.TokenTypeString))
		return
	}

	return &ast.LinkStatement{
		LinkToken: linkToken,
		Kind:      kind,
		Path:      path.(*ast.ValueExpression),
	}, true
}

func (p *Parser) eof() (ok bool) {
	if _, ok = p.expectToken(scanner.TokenTypeEOF); !ok {
		p.unread()
	}
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
			break
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

	// Return the peeked tokens to the FRONT of the buffer. Appending them
	// instead would reorder the stream whenever the buffer already holds
	// tokens (e.g. a macro expansion), corrupting subsequent parsing.
	p.returnToBuffer(tokens)
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
	p.errorAtToken(p.lastToken(), err)
}

// errorAtToken reports an error positioned at a specific token instead of
// whatever token happens to have been consumed last.
func (p *Parser) errorAtToken(token scanner.Token, err string) {
	if p.parserError == "" {
		p.parserError = err
		p.errorToken = token
	}
	if p.Error != nil {
		p.Error(p.readTokens-len(p.tokenBuffer), ast.StartPositionFromToken(token), ast.EndPositionFromToken(token), err)
	}
}

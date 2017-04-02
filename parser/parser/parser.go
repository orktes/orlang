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
				if isMacro {
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

func (p *Parser) processComment(tok scanner.Token) {
	comment := ast.Comment{Token: tok}
	if p.commentAfterNodeCheck != nil {
		node := p.commentAfterNodeCheck
		if comment.Token.EndLine == node.EndPos().Line {
			p.nodeComments[node] = append(p.nodeComments[node], comment)
			return
		}
	}
	p.comments = append(p.comments, comment)
}

func (p *Parser) checkCommentForNode(node ast.Node, afterNode bool) {
	if len(p.comments) == 0 {
		return
	}

	for i := len(p.comments) - 1; i >= 0; i-- {
		lastComment := p.comments[i]
		diff := node.StartPos().Line - lastComment.Token.EndLine

		if diff == 1 || diff == 0 {
			p.comments = append(p.comments[:i], p.comments[i+1:]...)
			p.nodeComments[node] = append(p.nodeComments[node], lastComment)
		} else if diff > 1 {
			break
		}
	}

	if afterNode {
		p.commentAfterNodeCheck = node
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

func (p *Parser) parseStatement(block bool) (node ast.Statement, ok bool) {
	ok = true
	var check = func(n ast.Statement, ok bool) bool {
		if ok {
			node = n
		}
		return ok
	}

	switch {
	case block && check(p.parseForLoop()):
	case block && check(p.parseIfStatement()):
	case check(p.parseMacroSubstitutionStatement()):
	case check(p.parseVarDecl()):
	default:
		ok = false
	}

	return
}

func (p *Parser) parseForLoop() (node *ast.ForLoop, nodeOk bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && token.Text == KeywordFor {
		nodeOk = true
		node = &ast.ForLoop{
			Start: ast.StartPositionFromToken(token),
		}
		var condition ast.Node
		var init ast.Node
		var after ast.Node

		init, statementok := p.parseStatementOrExpression(false) // Pre stuff
		token, ok := p.expectToken(scanner.TokenTypeSEMICOLON, scanner.TokenTypeLBRACE)
		if !ok {
			if statementok {
				p.error(unexpected(token.StringValue(), "; or code block"))
			} else {
				p.error(unexpected(token.StringValue(), "statement, ; or code block"))
			}

			return
		}

		if token.Type == scanner.TokenTypeLBRACE {
			p.unread()
			// TODO create isExpression to check if a node is an expression
			condition = init
			goto parseBlock
		}

		condition, statementok = p.parseExpression() // Condition
		if !statementok {
			p.error(unexpected(p.read().StringValue(), "expression"))
			return
		}
		token, ok = p.expectToken(scanner.TokenTypeSEMICOLON)
		if !ok {
			p.error(unexpected(token.StringValue(), ";"))
			return
		}

		after, _ = p.parseStatementOrExpression(false) // After

	parseBlock:
		block, blockOk := p.parseBlock() // Block
		if !blockOk {
			p.error(unexpected(p.read().StringValue(), "code block"))
			return
		}

		if condition != nil {
			node.Condition = condition.(ast.Expression)
		}

		node.Init = init
		node.After = after
		node.Block = block

		p.checkCommentForNode(node, false)

	} else {
		p.unread()
	}
	return
}

func (p *Parser) parseIfStatement() (node *ast.IfStatement, nodeOk bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && token.Text == KeywordIf {
		nodeOk = true
		node = &ast.IfStatement{
			Start: ast.StartPositionFromToken(token),
		}

		p.checkCommentForNode(node, false)

		condition, statementok := p.parseExpression() // Condition
		if !statementok {
			p.error(unexpected(p.read().StringValue(), "expression"))
			return
		}

		block, blockOk := p.parseBlock() // Block
		if !blockOk {
			p.error(unexpected(p.read().StringValue(), "code block"))
			return
		}

		node.Condition = condition
		node.Block = block

		token = p.read()
		if token.Type == scanner.TokenTypeIdent && token.Text == KeywordElse {
			var elblock *ast.Block

			elif, elseOk := p.parseIfStatement()
			if elseOk {
				elblock = &ast.Block{
					Start: ast.StartPositionFromToken(token),
					End:   elif.EndPos(),
					Body:  []ast.Node{elif},
				}
			} else if elblock, elseOk = p.parseBlock(); !elseOk {
				p.error(unexpected(p.read().StringValue(), "if statement or code block"))
			}

			node.Else = elblock
		} else {
			p.unread()
		}
	} else {
		p.unread()
	}
	return
}

func (p *Parser) parseAssigment(left ast.Expression) (node ast.Expression, ok bool) {
	_, ok = p.expectToken(scanner.TokenTypeASSIGN)
	if !ok {
		p.unread()
		return
	}

	expression, exprOk := p.parseExpression()
	if !exprOk {
		p.error(unexpected(p.read().StringValue(), "expression"))
		return
	}

	node = &ast.Assigment{Left: left, Right: expression}
	return
}

func (p *Parser) parseMemberExpression(target ast.Expression) (node *ast.MemberExpression, ok bool) {
	_, ok = p.expectToken(scanner.TokenTypePERIOD)
	if !ok {
		p.unread()
		return
	}

	token, propertyOk := p.expectToken(scanner.TokenTypeIdent)
	if !propertyOk {
		p.error(unexpected(token.StringValue(), "property name"))
		return
	}

	if isKeyword(token.Text) {
		p.error(reservedKeywordError(token))
		return
	}

	node = &ast.MemberExpression{
		Target:   target,
		Property: token,
	}

	return
}

func (p *Parser) parseCallExpression(target ast.Expression) (node *ast.FunctionCall, ok bool) {
	_, ok = p.expectToken(scanner.TokenTypeLPAREN)
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

	token, rParenOk := p.expectToken(scanner.TokenTypeRPAREN)
	if !rParenOk {
		p.error(unexpectedToken(token, scanner.TokenTypeRPAREN))
		return
	}

	node = &ast.FunctionCall{
		Callee:    target,
		Arguments: args,
		End:       ast.EndPositionFromToken(token),
	}

	return
}

func (p *Parser) parseCallArgument() (arg *ast.CallArgument, ok bool) {
	arg = &ast.CallArgument{}
	p.snapshot()
	tokens, namedArgument := p.expectPattern(scanner.TokenTypeIdent, scanner.TokenTypeCOLON)
	if namedArgument {
		p.commit()
		if isKeyword(tokens[0].Text) {
			p.error(reservedKeywordError(tokens[0]))
			return
		}
		arg.Name = &tokens[0]
	} else {
		p.restore()
	}

	expr, ok := p.parseExpression()
	if ok {
		arg.Expression = expr
		p.checkCommentForNode(arg, true)
	}

	return
}

func (p *Parser) parseType() (token scanner.Token, ok bool) {
	token, ok = p.expectToken(scanner.TokenTypeIdent)
	if !ok {
		p.unread()
		return
	}

	if isKeyword(token.Text) {
		p.error(reservedKeywordError(token))
	}

	return
}

func (p *Parser) parseValueExpression() (expression ast.Expression, ok bool) {
	var token scanner.Token
	if token, ok = p.expectToken(valueTypes...); !ok {
		p.unread()
		return
	}

	if isKeyword(token.Text) {
		p.error(reservedKeywordError(token))
		return
	}

	return &ast.ValueExpression{Token: token}, true
}

func (p *Parser) parseUnaryExpression() (expression ast.Expression, ok bool) {
	token, prefixOk := p.expectToken(unaryPrefix...)
	if prefixOk {
		var rExpr ast.Expression
		rExpr, ok = p.parseUnaryExpression()
		if !ok {
			p.error(unexpected(p.read().StringValue(), "expression"))
			return
		}
		expression = &ast.UnaryExpression{
			Operator:   token,
			Expression: rExpr,
		}
		return
	}

	p.unread()

	check := func(expr ast.Expression, cok bool) bool {
		if cok {
			ok = cok
			expression = expr
		}

		return cok
	}

	switch {
	case check(p.parseFuncDecl()):
	case check(p.parseValueExpression()):
	case check(p.parseMacroSubstitutionExpression()):
	default:
		return
	}

rightLoop:
	for {
		// Parse function calls, member expressions and type casts
		switch {
		case check(p.parseAssigment(expression)):
		case check(p.parseCallExpression(expression)):
		case check(p.parseMemberExpression(expression)):
		case check(p.parseComparisonExpression(expression)):
		default:
			break rightLoop
		}
	}

	if ok {
		token, suffixOk := p.expectToken(unarySuffix...)
		if suffixOk {
			expression = &ast.UnaryExpression{
				Operator:   token,
				Expression: expression,
			}
			return
		}

		p.unread()
	}

	return
}

func (p *Parser) parseBinaryExpression(left ast.Expression) (node *ast.BinaryExpression, ok bool) {
	token, ok := p.expectToken(
		scanner.TokenTypeADD,
		scanner.TokenTypeSUB,
		scanner.TokenTypeASTERISK,
		scanner.TokenTypeSLASH,
	)

	if !ok {
		p.unread()
		return
	}

	var right ast.Expression
	var exprOk bool
	switch token.Type {
	case scanner.TokenTypeASTERISK, scanner.TokenTypeSLASH:
		right, exprOk = p.parseUnaryExpression()
	default:
		right, exprOk = p.parseExpression()
	}

	if !exprOk {
		p.error(unexpected(p.read().StringValue(), "expression"))
		return
	}

	node = &ast.BinaryExpression{
		Operator: token,
		Left:     left,
		Right:    right,
	}

	return
}

func (p *Parser) parseExpression() (expression ast.Expression, ok bool) {
	if expression, ok = p.parseUnaryExpression(); ok {
		for {
			if binaryExpression, binaryOk := p.parseBinaryExpression(expression); binaryOk {
				expression = binaryExpression
			} else {
				break
			}
		}
	}
	return
}

func (p *Parser) parseComparisonExpression(left ast.Expression) (node ast.Expression, ok bool) {
	token, ok := p.expectToken(
		scanner.TokenTypeEqual,
		scanner.TokenTypeNotEqual,
		scanner.TokenTypeLess,
		scanner.TokenTypeGreater,
		scanner.TokenTypeLessOrEqual,
		scanner.TokenTypeGreaterOrEqual,
	)

	if !ok {
		p.unread()
		return
	}

	right, expressionOk := p.parseExpression()
	if !expressionOk {
		p.error(unexpected(p.read().StringValue(), "expression"))
		return
	}

	node = &ast.ComparisonExpression{
		Left:     left,
		Right:    right,
		Operator: token,
	}

	return
}

func (p *Parser) parseVarDecl() (node ast.Statement, ok bool) {
	token := p.read()
	if token.Type == scanner.TokenTypeIdent && (token.Text == KeywordVar || token.Text == KeywordConst) {
		ok = true

		isConstant := token.Text == KeywordConst
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
				p.error(unexpected(p.read().StringValue(), "variable declaration"))
				return
			}

			node = declaration
		}
	} else {
		p.unread()
	}
	return
}

func (p *Parser) parseVariableDeclarations(isConstant bool) (varDecls []*ast.VariableDeclaration, ok bool) {
	if t, lparenOk := p.expectToken(scanner.TokenTypeLPAREN); !lparenOk {
		p.error(unexpectedToken(t, scanner.TokenTypeLPAREN))
		return
	}

	for {
		var foundVarDecl = false
		var varDecl *ast.VariableDeclaration
		varDecl, ok = p.parseVariableDeclaration(isConstant)
		if ok {
			foundVarDecl = true
			varDecls = append(varDecls, varDecl)
		}

		var token scanner.Token

		var foundVarDeclEnd bool
		if foundVarDecl {
			token, foundVarDeclEnd = p.expectToken(scanner.TokenTypeRPAREN, scanner.TokenTypeCOMMA)
		} else {
			token, foundVarDeclEnd = p.expectToken(scanner.TokenTypeRPAREN)
		}

		if !foundVarDeclEnd {
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

func (p *Parser) parseVariableDeclaration(isConstant bool) (varDecl *ast.VariableDeclaration, ok bool) {
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

	varDecl = &ast.VariableDeclaration{}
	varDecl.Constant = isConstant
	varDecl.Name = token
	defer p.checkCommentForNode(varDecl, true)

	token, assignOk := p.expectToken(scanner.TokenTypeCOLON, scanner.TokenTypeASSIGN)
	if !assignOk {
		p.error(unexpectedToken(token, scanner.TokenTypeCOLON, scanner.TokenTypeASSIGN))
		return
	}

	if token.Type == scanner.TokenTypeCOLON {
		token, colonOk := p.parseType()
		if !colonOk {
			p.error(unexpectedToken(p.read(), scanner.TokenTypeIdent))
			return
		}

		varDecl.Type = token

		if _, defaultAssOk := p.expectToken(scanner.TokenTypeASSIGN); !defaultAssOk {
			p.unread()
			return
		}

	}

	expr, expressionOk := p.parseExpression()
	if !expressionOk {
		p.error(unexpected(p.read().StringValue(), "expression"))
		return
	}

	varDecl.DefaultValue = expr

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
		if pattrn, ok := p.parseMacroCall(token); pattrn != nil && ok {
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
	panic("No token in buffer")
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

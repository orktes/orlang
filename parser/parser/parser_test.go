package parser

import (
	"strings"
	"testing"

	"github.com/orktes/orlang/parser/ast"
	"github.com/orktes/orlang/parser/scanner"
)

func testScanner(src string) *scanner.Scanner {
	return scanner.NewScanner(strings.NewReader(src))
}

func TestParserRead(t *testing.T) {
	p := NewParser(testScanner("foobar"))

	if p.read().Text != "foobar" {
		t.Error("Wrong token returned")
	}

	if p.read().Type != scanner.TokenTypeEOF {
		t.Error("EOF should have been returned")
	}

}

func TestParserUnreadRead(t *testing.T) {
	p := NewParser(testScanner("foobar"))

	if p.read().Text != "foobar" {
		t.Error("Wrong token returned")
	}

	p.unread()

	if p.read().Text != "foobar" {
		t.Error("Wrong token returned")
	}
}

func TestParserPeek(t *testing.T) {
	p := NewParser(testScanner("foobar"))

	if p.peek().Text != "foobar" {
		t.Error("Wrong token returned")
	}

	if p.peek().Text != "foobar" {
		t.Error("Wrong token returned")
	}

	if p.read().Text != "foobar" {
		t.Error("Wrong token returned")
	}
}

func TestParserPeekMultiple(t *testing.T) {
	p := NewParser(testScanner("foobar;barfoo;"))

	tokens := p.peekMultiple(3)

	if tokens[0].Text != "foobar" {
		t.Error("Wrong token returned")
	}

	if tokens[1].Text != ";" {
		t.Error("Wrong token returned", tokens[1].String())
	}

	if tokens[2].Text != "barfoo" {
		t.Error("Wrong token returned", tokens[2].String())
	}

	if p.read().Text != "foobar" {
		t.Error("Wrong token returned")
	}

	if p.read().Text != ";" {
		t.Error("Wrong token returned")
	}
}

func TestParserSkip(t *testing.T) {
	p := NewParser(testScanner("foobar;barfoo;"))
	p.skip()
	if p.read().Text != ";" {
		t.Error("Wrong token returned")
	}
}

func TestLastTokenWithEmptyBuffer(t *testing.T) {
	p := NewParser(testScanner("foobar;barfoo;"))
	defer func() {
		if r := recover(); r == nil {
			t.Error("Should have paniced")
		}
	}()

	p.lastToken()
}

func TestParseVariableDeclarationsFailure(t *testing.T) {
	p := NewParser(testScanner("foo:bar)"))
	_, ok := p.parseVariableDeclarations(true)
	if ok {
		t.Error("Should not be able to parse")
	}
}

func TestExpectPattern(t *testing.T) {
	p := NewParser(testScanner("foobar;barfoo;"))
	tokens, ok := p.expectPattern(
		scanner.TokenTypeIdent,
		scanner.TokenTypeSEMICOLON,
		scanner.TokenTypeIdent,
		scanner.TokenTypeSEMICOLON)

	if !ok {
		t.Error("Didnt get expected pattern")
	}

	if tokens[0].Type != scanner.TokenTypeIdent {
		t.Error("Didnt get expected pattern")
	}

	if tokens[1].Type != scanner.TokenTypeSEMICOLON {
		t.Error("Didnt get expected pattern")
	}

	if tokens[2].Type != scanner.TokenTypeIdent {
		t.Error("Didnt get expected pattern")
	}

	if tokens[3].Type != scanner.TokenTypeSEMICOLON {
		t.Error("Didnt get expected pattern")
	}

	_, ok = p.expectPattern(scanner.TokenTypeIdent)
	if ok {
		t.Error("Nothing should be returned")
	}

}

func TestReturnToBuffer(t *testing.T) {
	p := NewParser(testScanner("foobar;barfoo;"))
	p.read()

	tokens, _ := p.expectPattern(
		scanner.TokenTypeSEMICOLON,
		scanner.TokenTypeIdent,
		scanner.TokenTypeSEMICOLON)

	p.returnToBuffer(tokens)

	tokens, ok := p.expectPattern(
		scanner.TokenTypeSEMICOLON,
		scanner.TokenTypeIdent,
		scanner.TokenTypeSEMICOLON)

	if !ok {
		t.Error("Didnt get expected pattern")
	}

	if tokens[0].Type != scanner.TokenTypeSEMICOLON {
		t.Error("Didnt get expected pattern")
	}

	if tokens[1].Type != scanner.TokenTypeIdent {
		t.Error("Didnt get expected pattern")
	}

	if tokens[2].Type != scanner.TokenTypeSEMICOLON {
		t.Error("Didnt get expected pattern")
	}

}

func TestFuncParse(t *testing.T) {
	file, err := Parse(strings.NewReader(`
    fn test(bar : int, foo : float = 0.2) {}
    fn withoutArguments() {}
  `))

	if err != nil {
		t.Error(err)
	}

	val, ok := file.Body[0].(*ast.FunctionDeclaration)
	if !ok {
		t.Error("Wrong type")
	}

	if len(val.Arguments) != 2 {
		t.Error("Wrong number of arguments")
	}

	if val.Arguments[0].Name.Text != "bar" {
		t.Error("Wrong argument name")
	}

	if val.Arguments[0].Type.Text != "int" {
		t.Error("Wrong argument name")
	}

	if val.Arguments[1].Name.Text != "foo" {
		t.Error("Wrong argument name")
	}

	if val.Arguments[1].Type.Text != "float" {
		t.Error("Wrong argument name")
	}

	if val.Arguments[1].DefaultValue.(*ast.ValueExpression).Value != 0.2 {
		t.Error("Wrong argument default value")
	}

	val, ok = file.Body[1].(*ast.FunctionDeclaration)
	if !ok {
		t.Error("Wrong type")
	}
	if len(val.Arguments) != 0 {
		t.Error("Wrong number of arguments")
	}
}

func TestParseFunctionAsDefaultValue(t *testing.T) {
	file, err := Parse(strings.NewReader(`
		fn foobar(foo : bar = fn () {
		}) {
		}
	`))

	if err != nil {
		t.Error(err)
	}

	val, ok := file.Body[0].(*ast.FunctionDeclaration)
	if !ok {
		t.Error("Wrong type")
	}

	_, ok = val.Arguments[0].DefaultValue.(*ast.FunctionDeclaration)
	if !ok {
		t.Error("Wrong type")
	}
}

func TestParseFunctionInsideFunction(t *testing.T) {
	file, err := Parse(strings.NewReader(`
		fn foobar() {
			fn barfoo() {
			}
		}
	`))

	if err != nil {
		t.Error(err)
	}

	val, ok := file.Body[0].(*ast.FunctionDeclaration)
	if !ok {
		t.Error("Wrong type")
	}

	nestedFunction, ok := val.Block.Body[0].(*ast.FunctionDeclaration)
	if !ok {
		t.Error("Wrong type")
	}

	if nestedFunction.Name.Text != "barfoo" {
		t.Error("Nested function name is wrong")
	}
}

func TestParseForLoop(t *testing.T) {
	_, err := Parse(strings.NewReader(`
		fn foobar() {
			for var i = 0; true; {
			}

			for foo; bar; baz {
			}

			for foo; bar; {
			}

			for ; bar ; {
			}

			for true {

			}

			for {

			}
		}
	`))

	if err != nil {
		t.Error(err)
	}
}

func TestParseAssignment(t *testing.T) {
	_, err := Parse(strings.NewReader(`
		fn foobar() {
			var foo = 123;
			foo = 124;
		}
	`))

	if err != nil {
		t.Error(err)
	}
}

func TestParseIfCondition(t *testing.T) {
	_, err := Parse(strings.NewReader(`
		fn foobar() {
			if true {}
		}
	`))

	if err != nil {
		t.Error(err)
	}
}

func TestParseIfElseCondition(t *testing.T) {
	_, err := Parse(strings.NewReader(`
		fn foobar() {
			if true {} else {}
		}
	`))

	if err != nil {
		t.Error(err)
	}
}

func TestParseIfElseIfCondition(t *testing.T) {
	_, err := Parse(strings.NewReader(`
		fn foobar() {
			if true {} else if false {}
		}
	`))

	if err != nil {
		t.Error(err)
	}
}

func TestParseVariableDeclarationInsideFunction(t *testing.T) {
	_, err := Parse(strings.NewReader(`
		fn foobar() {
			var foo = bar;
			var barfoo : int = 123;
		}
	`))

	if err != nil {
		t.Error(err)
	}
}

func TestParseVariableDeclaration(t *testing.T) {
	file, err := Parse(strings.NewReader(`
		var foo : Bar
		const bar = 123
	`))

	if err != nil {
		t.Error(err)
	}

	val, ok := file.Body[0].(*ast.VariableDeclaration)
	if !ok {
		t.Error("Wrong type")
	}

	if val.Name.Text != "foo" || val.Type.Text != "Bar" {
		t.Error("Type could not be parsed")
	}

	val, ok = file.Body[1].(*ast.VariableDeclaration)
	if !ok {
		t.Error("Wrong type")
	}

	if val.Name.Text != "bar" || !val.Constant {
		t.Error("Type could not be parsed")
	}

}

func TestParseMultipleVariableDeclarations(t *testing.T) {
	file, err := Parse(strings.NewReader(`
		var (
			foo : Bar,
			bar : int,
			biz : float = 123,
			boz = 123
		)
	`))
	if err != nil {
		t.Error(err)
	}

	val, ok := file.Body[0].(*ast.MultiVariableDeclaration)
	if !ok {
		t.Error("Wrong type")
	}

	if val.Declarations[0].Name.Text != "foo" || val.Declarations[0].Type.Text != "Bar" {
		t.Error("Type could not be parsed")
	}

	if val.Declarations[1].Name.Text != "bar" || val.Declarations[1].Type.Text != "int" {
		t.Error("Type could not be parsed")
	}

	if val.Declarations[2].Name.Text != "biz" || val.Declarations[2].Type.Text != "float" {
		t.Error("Type could not be parsed")
	}

	if val.Declarations[2].DefaultValue.(*ast.ValueExpression).Value != int64(123) {
		t.Error("Wrong default value")
	}

	if val.Declarations[3].Name.Text != "boz" {
		t.Error("Type could not be parsed")
	}

	if val.Declarations[3].DefaultValue.(*ast.ValueExpression).Value != int64(123) {
		t.Error("Wrong default value")
	}
}

func TestParseFailures(t *testing.T) {
	tests := []struct {
		src string
		err string
	}{
		// Undefined token
		{"unexpected", "1:1: Unexpected token IDENT"},
		{"bar () {}", "1:1: Unexpected token IDENT"},
		// Invalid functions
		{"fn test{}", "1:9: Expected [LPAREN] got LBRACE"},
		{"fn test()", "1:10: Expected code block got EOF"},
		{"fn test(,) {}", "1:10: Expected [IDENT RPAREN] got COMMA"},
		{"fn test(bar , int, foo : float = 0.2) {}", "1:18: Expected [COLON ASSIGN] got COMMA"},
		{"fn test(foo : float foo) {}", "1:24: Expected [RPAREN COMMA] got foo"},
		{"fn test(foo : ) {}", "1:18: Expected [IDENT] got RPAREN"},
		{"fn test(foo : bar = ) {}", "1:24: Expected expression got RPAREN"},
		{"fn test(foo : int) {]", "1:21: Expected code block got RBRACK"},
		{"fn", "1:3: Expected [IDENT LPAREN] got EOF"},
		{"fn (foo : int) {}", "1:17: Root level functions can't be anonymous"},
		// Variable declarations
		{"var [", "1:5: Expected variable declaration got LBRACK"},
		{"var (bar , int, foo : float = 0.2)", "1:12: Expected [COLON ASSIGN] got COMMA"},
		{"var (foo : float foo)", "1:18: Expected [RPAREN COMMA] got foo"},
		{"var (foo : )", "1:13: Expected [IDENT] got RPAREN"},
		{"var (foo : bar = )", "1:19: Expected expression got RPAREN"},
		{"fn foobar() { var foobar : int }", "1:33: Expected [SEMICOLON] got RBRACE"},
		// For loops
		{"fn foobar() { for var i = 0; i; [] }", "1:34: Expected code block got LBRACK"},
		{"fn foobar() { for var i = 0; {}}", "1:31: Expected expression got LBRACE"},
		{"fn foobar() { for var i = 0; true {}}", "1:36: Expected ; got LBRACE"},
		{"fn foobar() { for }", "1:20: Expected statement, ; or code block got RBRACE"},
		{"fn foobar() { for true true {} }", "1:29: Expected ; or code block got BOOL"},
		{"fn foobar() { foo = 123 }", "1:26: Expected [SEMICOLON] got RBRACE"},
		{"fn foobar() { foo = , }", "1:23: Expected expression got COMMA"},
		// If statemts
		{"fn foobar() {  if }", "1:20: Expected expression got RBRACE"},
		{"fn foobar() {  if true foo }", "1:28: Expected code block got IDENT"},
		{"fn foobar() {  if true {} else f", "1:33: Expected if statement or code block got IDENT"},
	}

	for _, test := range tests {
		_, err := Parse(strings.NewReader(test.src))

		if err == nil {
			t.Errorf("Expected %s to return error %s, but got nothing", test.src, test.err)
		}

		if err.Error() != test.err {
			t.Errorf("Expected %s to return error %s, but got %s", test.src, test.err, err.Error())
		}
	}
}

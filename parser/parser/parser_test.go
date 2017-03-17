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

func TestFuncParseFailure1(t *testing.T) {
	_, err := Parse(strings.NewReader(`
    fn test(bar , int, foo : float = 0.2) {}
  `))

	if err == nil {
		t.Error("Should have returned an error")
	}

	if err.Error() != "1:21: Expected [COLON] got COMMA" {
		t.Error(err)
	}
}

func TestFuncParseFailure2(t *testing.T) {
	_, err := Parse(strings.NewReader(`
    fn test(,) {}
  `))

	if err == nil {
		t.Error("Should have returned an error")
	}

	if err.Error() != "1:13: Expected [IDENT RPAREN] got COMMA" {
		t.Error(err)
	}
}

func TestFuncParseFailure3(t *testing.T) {
	_, err := Parse(strings.NewReader(`
    fn test()
  `))

	if err == nil {
		t.Error("Should have returned an error")
	}

	if err.Error() != "2:2: Expected [LBRACE] got EOF" {
		t.Error(err)
	}
}

func TestFuncParseFailure4(t *testing.T) {
	_, err := Parse(strings.NewReader(`
    fn test{}
  `))

	if err == nil {
		t.Error("Should have returned an error")
	}

	if err.Error() != "1:12: Expected [LPAREN] got LBRACE" {
		t.Error(err)
	}
}

func TestParseFailureUnexpected(t *testing.T) {
	_, err := Parse(strings.NewReader(`
    unexpected
  `))

	if err == nil {
		t.Error("Should have returned an error")
	}

	if err.Error() != "1:4: Unexpected token IDENT" {
		t.Error(err)
	}
}

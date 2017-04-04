package parser

import (
	"fmt"
	"testing"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

func testCreateMacroMatcher(str string) *macroMatcher {
	p := NewParser(testScanner(fmt.Sprintf(`
    macro M {
      %s
    }
  `, str)))

	macro, _ := p.parseMacro()
	matcher := newMacroMatcher(macro)
	return matcher
}

func TestMacroMatcherWithSinglePattern(t *testing.T) {
	matcher := testCreateMacroMatcher(`
    ($foo:expr) : ()
  `)

	if !matcher.acceptsType("expr") {
		t.Error("Should accept expr")
	}
	if matcher.acceptsType("stmt") {
		t.Error("Should accept expr")
	}
	matcher.feed(&ast.MemberExpression{})
	if matcher.acceptsType("expr") {
		t.Error("Should accept expr")
	}

	ptrn := matcher.match()
	if ptrn == nil {
		t.Error("Should have returned a matching pattern")
	}

}

func TestMacroMatcherWithMultiplePatterns(t *testing.T) {
	matcher := testCreateMacroMatcher(`
    ($foo:expr) : ()
    ($bar:stmt) : ()
  `)

	if !matcher.acceptsType("expr") {
		t.Error("Should accept expr")
	}
	if !matcher.acceptsType("stmt") {
		t.Error("Should accept expr")
	}
	matcher.feed(&ast.MemberExpression{})

	ptrn := matcher.match().pattern
	if ptrn == nil {
		t.Error("Should have returned a matching pattern")
	}

	if ptrn.Pattern[0].(*ast.MacroMatchArgument).Name != "$foo" {
		t.Error("Wrong pattern returned", ptrn.Pattern[0].(*ast.MacroMatchArgument))
	}

	matcher = testCreateMacroMatcher(`
    ($foo:expr) : ()
    ($foo:stmt) : ()
  `)

	if matcher.match() != nil {
		t.Error("Nothing matches empty")
	}

	matcher = testCreateMacroMatcher(`
    () : ()
    ($foo:stmt) : ()
  `)

	if matcher.match() == nil {
		t.Error("First pattern should match empty")
	}
}

func TestMacroMatcherWithRepetitionWithDelimiter(t *testing.T) {
	matcher := testCreateMacroMatcher(`
    ($($foo:expr),*) : ()
  `)

	if !matcher.acceptsType("expr") {
		t.Error("Should accept expr")
	}
	matcher.feed(&ast.ValueExpression{})
	if matcher.acceptsType("expr") {
		t.Error("Should wait for delimiter ,")
	}
	matcher.feed(scanner.Token{Type: scanner.TokenTypeCOMMA, Text: ","})
	if !matcher.acceptsType("expr") {
		t.Error("Should have accepted a expr")
	}
}

func TestMacroMatcherWithAtMostOnceRepetition(t *testing.T) {
	matcher := testCreateMacroMatcher(`
    ($($foo:expr)? foo) : ()
  `)

	if !matcher.acceptsType("token") {
		t.Error("Should accept token")
	}

	if !matcher.acceptsType("expr") {
		t.Error("Should accept expr")
	}

	matcher.feed(&ast.ValueExpression{})
	if matcher.acceptsType("expr") {
		t.Error("Should not accept another one")
	}

	if !matcher.acceptsType("token") {
		t.Error("Should accept token")
	}

	matcher.feed(scanner.Token{
		Type:  scanner.TokenTypeIdent,
		Text:  "foo",
		Value: "foo",
	})

	if matcher.match() == nil {
		t.Error("Match should be found")
	}
}

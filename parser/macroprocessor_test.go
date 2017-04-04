package parser

import (
	"testing"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

func TestMacroProcessorWithManualAdvance(t *testing.T) {
	p := NewParser(testScanner("(foo $foo:expr)"))
	patterns, _ := p.parseMacroMatchPatterns()

	mp := newMacroPreprocessor(patterns, false)
	if !mp.acceptsType("token") {
		t.Error("Should accept token")
	}
	mp.currentPos++
	if !mp.acceptsType("expr") {
		t.Error("Should accept expression")
	}
	if mp.acceptsType("stmt") {
		t.Error("Should not accept statement")
	}
}

func TestMacroProcessorWithManualAdvanceAndRepetition(t *testing.T) {
	p := NewParser(testScanner("($($bar:stmt)* $foo:expr)"))
	patterns, _ := p.parseMacroMatchPatterns()

	mp := newMacroPreprocessor(patterns, false)
	if !mp.acceptsType("stmt") {
		t.Error("Should accept stmt")
	}
	if !mp.acceptsType("expr") {
		t.Error("Should accept expression")
	}

	// Require one match
	p = NewParser(testScanner("($($bar:stmt)+ $foo:expr)"))
	patterns, _ = p.parseMacroMatchPatterns()
	mp = newMacroPreprocessor(patterns, false)
	if !mp.acceptsType("stmt") {
		t.Error("Should accept stmt")
	}
	if mp.acceptsType("expr") {
		t.Error("Should not accept expression before currentPos is increased")
	}

	mp.currentPos++
	if !mp.acceptsType("expr") {
		t.Error("Should not accept expression before currentPos is increased")
	}
}

func TestMacroProcessorFeed(t *testing.T) {
	p := NewParser(testScanner("($($bar:stmt foo)+ $foo:expr)"))
	patterns, _ := p.parseMacroMatchPatterns()
	mp := newMacroPreprocessor(patterns, false)

	if mp.ok() {
		t.Error("Not even started")
	}

	if mp.acceptsType("expr") {
		t.Error("Should not accept expr before one statement has been succefully captured")
	}

	if !mp.feed(&ast.IfStatement{}) {
		t.Error("Should feed in if statement")
	}
	if mp.subProcessors[patterns[0]].currentPos != 1 {
		t.Error("Should have advanced")
	}
	if mp.ok() {
		t.Error("Should not be okay")
	}
	if !mp.feed(scanner.Token{
		Type:  scanner.TokenTypeIdent,
		Text:  "foo",
		Value: "foo",
	}) {
		t.Error("Should feed in foo")
	}
	if mp.currentPos != 0 {
		t.Error("Should not have advanced")
	}

	if !mp.acceptsType("expr") {
		t.Error("Should already accept expr")
	}

	if !mp.feed(&ast.IfStatement{}) {
		t.Error("Should feed in if statement")
	}
	if !mp.feed(scanner.Token{
		Type:  scanner.TokenTypeIdent,
		Text:  "foo",
		Value: "foo",
	}) {
		t.Error("Should feed in foo")
	}
	if mp.currentPos != 0 {
		t.Error("Should not have advanced")
	}

	if !mp.acceptsType("expr") {
		t.Error("Should already accept expr")
	}

	if !mp.feed(&ast.FunctionCall{
		Callee: &ast.ValueExpression{Token: scanner.Token{Text: "foo"}},
	}) {
		t.Error("Should feed in function call expression")
	}

	if !mp.ok() {
		t.Error("Processor should be ok")
	}

	if mp.feed(&ast.FunctionCall{}) {
		t.Error("Should only allow one expression")
	}

	if mp.feed(&ast.FunctionCall{}) {
		t.Error("Should only allow one expression")
	}

	if mp.ok() {
		t.Error("Processor should not be ok after invalid value feed")
	}

	if len(mp.values["$foo"]) != 1 {
		t.Error("Expected to capture function")
	}

	if len(mp.subProcessors[patterns[0]].values["$bar"]) != 2 {
		t.Error("expected to capture the two if statements")
	}

	if mp.subProcessors[patterns[0]].get("$foo", 0).(*ast.FunctionCall).Callee.(*ast.ValueExpression).Token.Text != "foo" {
		t.Error("Get didnt return correct value from parent processor")
	}

	if mp.subProcessors[patterns[0]].get("$foo", 1).(*ast.FunctionCall).Callee.(*ast.ValueExpression).Token.Text != "foo" {
		t.Error("Get didnt return correct value from parent processor")
	}

	if _, ok := mp.subProcessors[patterns[0]].get("$bar", 1).(*ast.IfStatement); !ok {
		t.Error("Get didnt return correct value")
	}

	if mp.subProcessors[patterns[0]].loops != 2 {
		t.Error("Wrong number of loops")
	}
}

func TestMacroNestedRepetitions(t *testing.T) {
	p := NewParser(testScanner(`($(
			$($bar:expr)+
		)+)`))
	patterns, ok := p.parseMacroMatchPatterns()
	if !ok {
		t.Error("Should be able to parse")
	}
	mp := newMacroPreprocessor(patterns, false)
	if !mp.acceptsType("expr") {
		t.Error("Should accept expr")
	}
	if mp.ok() {
		t.Error("Outer repetitio is at least once")
	}
	if !mp.feed(&ast.FunctionCall{}) {
		t.Error("Should allow expr")
	}
	if !mp.ok() {
		t.Error("Should be okay after one item")
	}

	if !mp.feed(&ast.FunctionCall{}) {
		t.Error("Should allow expr")
	}
	if !mp.ok() {
		t.Error("Should be okay after two item")
	}
}

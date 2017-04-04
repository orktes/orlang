package parser

import (
	"strings"
	"testing"

	"github.com/orktes/orlang/parser/ast"
)

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

func TestParseVariableDeclarationInsideFunction(t *testing.T) {
	_, err := Parse(strings.NewReader(`
		fn foobar() {
			var foo = bar
			var barfoo : int = 123
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

func TestParseForLoop(t *testing.T) {
	_, err := Parse(strings.NewReader(`
		fn foobar() {
			for var i = 0; true; {
			}

			for foo; bar; baz {
			}

			for foo; 1 <= 2; {
			}

			for ; 1 < 2 ; {
			}

			for 1 == 1 {

			}

			for 1 != 1 {

			}

			for 1 > 1 {

			}

			for 1 >= 1 {

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
			var foo = 123
			foo = 124
		}
	`))

	if err != nil {
		t.Error(err)
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

	if nestedFunction.Identifier.Text != "barfoo" {
		t.Error("Nested function name is wrong")
	}
}

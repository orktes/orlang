package parser

import (
	"strings"
	"testing"

	"github.com/orktes/orlang/ast"
)

func TestParseVariableDeclaration(t *testing.T) {
	file, err := Parse(strings.NewReader(`
		var foo : Bar
		const bar = 123
		const biz : (int32, float32) => int32
		const arr : []int32
		const arrWithLength : [1]int32
		const fobauh : (int)
	`))

	if err != nil {
		t.Error(err)
	}

	val, ok := file.Body[0].(*ast.VariableDeclaration)
	if !ok {
		t.Error("Wrong type")
	}

	if val.Name.Text != "foo" || val.Type.(*ast.TypeReference).Token.Text != "Bar" {
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

			for var i = 0; i < 10; i++ {

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
		fn withoutArguments() { return }
  `))

	if err != nil {
		t.Error(err)
	}

	val, ok := file.Body[0].(*ast.FunctionDeclaration)
	if !ok {
		t.Error("Wrong type")
	}

	if len(val.Signature.Arguments) != 2 {
		t.Error("Wrong number of arguments")
	}

	if val.Signature.Arguments[0].Name.Text != "bar" {
		t.Error("Wrong argument name")
	}

	if val.Signature.Arguments[0].Type.(*ast.TypeReference).Token.Text != "int" {
		t.Error("Wrong argument name")
	}

	if val.Signature.Arguments[1].Name.Text != "foo" {
		t.Error("Wrong argument name")
	}

	if val.Signature.Arguments[1].Type.(*ast.TypeReference).Token.Text != "float" {
		t.Error("Wrong argument name")
	}

	if val.Signature.Arguments[1].DefaultValue.(*ast.ValueExpression).Value != 0.2 {
		t.Error("Wrong argument default value")
	}

	val, ok = file.Body[1].(*ast.FunctionDeclaration)
	if !ok {
		t.Error("Wrong type")
	}
	if len(val.Signature.Arguments) != 0 {
		t.Error("Wrong number of arguments")
	}

	if _, ok := val.Block.Body[0].(*ast.ReturnStatement); !ok {
		t.Error("Could not find return statement")
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

	_, ok = val.Signature.Arguments[0].DefaultValue.(*ast.FunctionDeclaration)
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

	if nestedFunction.Signature.Identifier.Text != "barfoo" {
		t.Error("Nested function name is wrong")
	}
}

func TestParseFunctionReturnStatement(t *testing.T) {
	file, err := Parse(strings.NewReader(`
		fn foobar() {
			return foo
		}
	`))

	if err != nil {
		t.Error(err)
	}

	val, ok := file.Body[0].(*ast.FunctionDeclaration)
	if !ok {
		t.Error("Wrong type")
	}

	returnStmt, ok := val.Block.Body[0].(*ast.ReturnStatement)
	if !ok {
		t.Error("Wrong type")
	}

	if returnStmt.Expression.(*ast.Identifier).Text != "foo" {
		t.Error("Wrong expression found")
	}
}

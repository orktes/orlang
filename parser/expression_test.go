package parser

import (
	"strings"
	"testing"

	"github.com/orktes/orlang/ast"
)

func TestParseUnaryExpression(t *testing.T) {
	_, err := Parse(strings.NewReader(`
		fn main() {
			var foo = -1
			var bar = +1
			var foobar = ++1
			foobar = foobar++
		}
	`))
	if err != nil {
		t.Error(err)
	}
}

func TestParseBinaryExpression(t *testing.T) {
	file, err := Parse(strings.NewReader(`
		fn main() {
			var foo = 1 + 2 * 3 + 4
		}
	`))
	if err != nil {
		t.Error(err)
	}

	binaryExpr, ok := file.Body[0].(*ast.FunctionDeclaration).Block.Body[0].(*ast.VariableDeclaration).DefaultValue.(*ast.BinaryExpression)
	if !ok {
		t.Error("Wrong type")
	}

	//fmt.Printf("%#v", binaryExpr)

	if binaryExpr.Left.(*ast.ValueExpression).Value != int64(1) {
		t.Error("Wrong value on the left most side")
	}

	binaryExprRight := binaryExpr.Right.(*ast.BinaryExpression)
	if binaryExprRight.Right.(*ast.ValueExpression).Value != int64(4) {
		t.Error("Wrong value on the right most side")
	}

	if binaryExprRight.Left.(*ast.BinaryExpression).Left.(*ast.ValueExpression).Value != int64(2) {
		t.Error("Wrong value on the inner left")
	}

	if binaryExprRight.Left.(*ast.BinaryExpression).Right.(*ast.ValueExpression).Value != int64(3) {
		t.Error("Wrong value on the inner right")
	}
}

func TestParseFunctionCall(t *testing.T) {
	file, err := Parse(strings.NewReader(`
		fn foobar(x : int = 0, y: int = 0) : (int, float) {
			foobar()
			foobar()()
			foobar()()()
			someObj.foo()
			foobar(10, 20)
			foobar(x: 10, y: 20)
		}
	`))

	if err != nil {
		t.Error(err)
	}

	val, ok := file.Body[0].(*ast.FunctionDeclaration)
	if !ok {
		t.Error("Wrong type")
	}

	functionCall, ok := val.Block.Body[0].(*ast.FunctionCall)
	if !ok {
		t.Error("Wrong type")
	}

	callee, ok := functionCall.Callee.(*ast.Identifier)
	if !ok {
		t.Error("Wrong type")
	}

	if callee.Text != "foobar" {
		t.Error("Wrong callee")
	}

	functionCall, ok = val.Block.Body[1].(*ast.FunctionCall)
	if !ok {
		t.Error("Wrong type")
	}

	callee = functionCall.Callee.(*ast.FunctionCall).Callee.(*ast.Identifier)
	if callee.Text != "foobar" {
		t.Error("Wrong callee")
	}

	functionCall, ok = val.Block.Body[2].(*ast.FunctionCall)
	if !ok {
		t.Error("Wrong type")
	}

	callee = functionCall.Callee.(*ast.FunctionCall).Callee.(*ast.FunctionCall).Callee.(*ast.Identifier)
	if callee.Text != "foobar" {
		t.Error("Wrong callee")
	}

	functionCall, ok = val.Block.Body[3].(*ast.FunctionCall)
	if !ok {
		t.Error("Wrong type")
	}

	memberExpression := functionCall.Callee.(*ast.MemberExpression)
	if memberExpression.Property.Text != "foo" {
		t.Error("Wrong callee")
	}

	if memberExpression.Target.(*ast.Identifier).Text != "someObj" {
		t.Error("Member expression was parsed incorrectly")
	}

	functionCall, ok = val.Block.Body[4].(*ast.FunctionCall)
	if !ok {
		t.Error("Wrong type")
	}

	callee, ok = functionCall.Callee.(*ast.Identifier)
	if !ok {
		t.Error("Wrong type")
	}

	if functionCall.Arguments[0].Expression.(*ast.ValueExpression).Value != int64(10) {
		t.Error("Wrong arg value")
	}

	if functionCall.Arguments[1].Expression.(*ast.ValueExpression).Value != int64(20) {
		t.Error("Wrong arg value")
	}

	functionCall, ok = val.Block.Body[5].(*ast.FunctionCall)
	if !ok {
		t.Error("Wrong type")
	}

	callee, ok = functionCall.Callee.(*ast.Identifier)
	if !ok {
		t.Error("Wrong type")
	}

	if functionCall.Arguments[0].Expression.(*ast.ValueExpression).Value != int64(10) {
		t.Error("Wrong arg value")
	}

	if functionCall.Arguments[0].Name.Text != "x" {
		t.Error("Wrong arg name")
	}

	if functionCall.Arguments[1].Expression.(*ast.ValueExpression).Value != int64(20) {
		t.Error("Wrong arg value")
	}

	if functionCall.Arguments[1].Name.Text != "y" {
		t.Error("Wrong arg name")
	}

}

package analyser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/orktes/orlang/ast"

	"github.com/orktes/orlang/parser"
)

func TestVisitor(t *testing.T) {
	file, err := parser.Parse(strings.NewReader(`
    fn foobar(x : int32, y : float32) : (float32, int32) {
      return (y, x)
    }

    fn main() {
			var bar = 1
			var biz = (bar, 2.0)
			biz = (1, 3.0)
			var fuz : (int32, float32) = biz
			var fiz = foobar(10, 2.0)
			fiz = (0.5,11)

			var namedArgs = foobar(y: 2.0, x: 1)

			var tupleVar = (1,1)
			tupleVar = (5, 5)

			var complex : ((int32, int32), int32)
			complex = ((1,1), 1)

			var ((foo1, foo2), foo3) : ((int32, int32), int32) = complex
			var (dsadas, dadsa) = (1, 2)

			foo3 = 1
			foo3 = foo3 + foo3

			var fnVar : (int32, float32) : (float32, int32)
			fnVar = foobar

			var arrVar : []int32
			var anotherArrVar : []int32 = arrVar
			var anotherArrVarWithLength : [2]int32 = arrVar // TODO Will this be PITA in the runtime ?
			anotherArrVarWithLength = []int32{1, 2}
			var initArrVar = []int32{1, 2}
			arrVar = initArrVar
			//var value : int32 = initArrVar[0]

			var boolValue : bool = true
			boolValue = false

			var strVal = ""
		}
  `))
	if err != nil {
		t.Error(err)
	}

	visitor := &visitor{
		scope: NewScope(file),
		node:  file,
		info:  NewFileInfo(),
		errorCb: func(node ast.Node, msg string, fatal bool) {
			if !fatal {
				return
			}
			t.Fatalf("%s %#v", msg, node)
		},
	}

	ast.Walk(visitor, file)
}

func TestVisitorErrors(t *testing.T) {
	tests := []struct {
		src string
		err string
	}{
		{"var foo : unknown = 1", "1:21 cannot use 1 (type int32) as type unknown (unknown) in assigment"},
		{"fn foo() { var foo : float32 = 1 }", "1:32 cannot use 1 (type int32) as type float32 in assigment"},
		{`
			fn foo() {
				var foo : int32 = -1
				foo = (0.5)
			}
		`, "4:11 cannot use 0.5 (type float32) as type int32 in assigment expression"},
		{`
			fn foo() {
				var tupleVar = (5, 5)
				tupleVar = (0.5, 0.5)
			}
		`, "4:16 cannot use (0.5, 0.5) (type (float32, float32)) as type (int32, int32) in assigment expression"},
		{`
			fn foo() {
				var notFn = 1
				notFn()
			}
		`, "4:5 notFn (type int32) is not a function"},
		{`
			fn foo() {
				return 1
			}
		`, "3:12 cannot use 1 (type int32) as type void in return statement"},
		{`
			fn foo() {
				1 + 0.5
			}
		`, "3:5 invalid operation: 1 + 0.5 (mismatched types int32 and float32)"},
		{`
			fn foo(x : int32 = 0.5) {
			}
		`, "2:23 cannot use 0.5 (type float32) as type int32 in assigment"},
		{`
			fn foo(x : int32, x : int32) {
			}
		`, "2:22 x already declared"},
		{`
			fn foo(x : int32) {
				var a = 1
				fn a(x : int32) {
				}
			}
		`, "4:5 a already declared"},
		{`
			fn foo(x : int32) {
				var (foo, bar) = 1
			}
		`, "3:22 cannot use 1 (type int32) as tuple"},
		{`
			fn foo(x : int32) {
				var (foo, bar) = (1, 1)
				foo = 0.5
			}
		`, "4:11 cannot use 0.5 (type float32) as type int32 in assigment expression"},
		{`
			fn foo(x : int32) {
				var (foo, bar) : (int32, int32) = (0.5, 0.5)
			}
		`, "3:39 cannot use (0.5, 0.5) (type (float32, float32)) as type (int32, int32) in assigment"},
		{`
			fn foo(x : int32) {
				var a = 1
				var a = 1.0
			}
		`, "4:9 a already declared"},
		{`
			fn foo(x : int32) {
				var a = 1
				var b = a
				b = 0.5
			}
		`, "5:9 cannot use 0.5 (type float32) as type int32 in assigment expression"},
		{`
			fn foo(x : int32) {
				var a : [0.5]int32 = []int32{1}
			}
		`, "3:14 array length must be an integer"},
		{`
			fn foo(x : int32) {
				foo()
			}
		`, "3:5 too few arguments in call to foo"},
		{`
			fn foo(x : int32) {
				foo(1, 2, 3, 4)
			}
		`, "3:5 too many arguments in call to foo"},
		{`
			fn foo(x : int32) {
				foo(x: 1.0)
			}
		`, "3:12 cannot use 1.0 (type float32) as type int32 in function call"},
		{`
			fn foo(x : int32) {
				foo(y: 1)
			}
		`, "3:9 called function has no argument named y"},
		{`
			fn foo(x : int32, y: int32) {
				foo(y: 1, 1)
			}
		`, "3:5 named and non-named call arguments cannot be mixed"},
		{`
			fn foo() {
				var bar = 1
			}
		`, "3:9 bar declared but not used"},
		{`
			fn foo() {
				bar
			}
		`, "3:5 undefined: bar"},
		{`
			fn foo(x: int32) {

			}
		`, "2:11 x declared but not used"},
		{`
			fn foo() {
				var foo = 1
			}
		`, "3:9 foo declared but not used"},
		{`
			fn foo() {
			}
		`, "2:7 foo declared but not used"},
	}

	for _, test := range tests {
		file, err := parser.Parse(strings.NewReader(test.src))
		if err != nil {
			t.Error(err)
		}

		var errStr string
		visitor := &visitor{
			scope: NewScope(file),
			node:  file,
			info:  NewFileInfo(),
			errorCb: func(node ast.Node, msg string, _ bool) {
				if errStr != "" {
					// TODO handle multiple errors
					return
				}

				errStr = fmt.Sprintf(
					"%d:%d %s",
					node.StartPos().Line+1,
					node.StartPos().Column+1,
					msg,
				)
			},
		}

		ast.Walk(visitor, file)

		if errStr != test.err {
			t.Errorf("Expected %s to return error %s, but got %s", test.src, test.err, errStr)
		}
	}
}

func BenchmarkVisitor(b *testing.B) {
	file, err := parser.Parse(strings.NewReader(`
    fn foobar(x : int32, y : float32) : (float32, int32) {
      return (y, x)
    }

    fn main() {
			var bar = 1
			var biz = (bar, 2.0)
			biz = (1, 3.0)
			var fuz : (int32, float32) = biz
			var fiz = foobar(10, 2.0)
			fiz = (0.5,11)

			var namedArgs = foobar(y: 2.0, x: 1)

			var tupleVar = (1,1)
			tupleVar = (5, 5)

			var complex : ((int32, int32), int32)
			complex = ((1,1), 1)

			var ((foo1, foo2), foo3) : ((int32, int32), int32) = complex
			var (dsadas, dadsa) = (1, 2)

			foo3 = 1
			foo3 = foo3 + foo3

			var fnVar : (int32, float32) : (float32, int32)
			fnVar = foobar

			var arrVar : []int32
			var anotherArrVar : []int32 = arrVar
			var anotherArrVarWithLength : [2]int32 = arrVar // TODO Will this be PITA in the runtime ?
			anotherArrVarWithLength = []int32{1, 2}
			var initArrVar = []int32{1, 2}
			arrVar = initArrVar
			//var value : int32 = initArrVar[0]

			var boolValue : bool = true
			boolValue = false

			var strVal = ""
		}
  `))
	if err != nil {
		b.Error(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	visitor := &visitor{
		scope: NewScope(file),
		node:  file,
		info:  NewFileInfo(),
		errorCb: func(node ast.Node, msg string, fatal bool) {
			if !fatal {
				return
			}
			b.Fatalf("%s %#v", msg, node)
		},
	}

	for i := 0; i < b.N; i++ {
		visitor.scope = NewScope(file)
		ast.Walk(visitor, file)
	}
}

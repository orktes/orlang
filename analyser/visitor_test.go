package analyser

import (
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

			var complex : ((int32, int32), int32)
			complex = ((1,1), 1)

			var ((foo1, foo2), foo3) : ((int32, int32), int32) = complex

			foo3 = 1

    }
  `))
	if err != nil {
		t.Error(err)
	}

	visitor := &visitor{
		scope: NewScope(),
		node:  file,
		info:  &FileInfo{},
		errorCb: func(node ast.Node, msg string, bool bool) {
			t.Fatalf("%s %#v", msg, node)
		},
	}

	ast.Walk(visitor, file)
}

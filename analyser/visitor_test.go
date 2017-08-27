package analyser

import (
	"strings"
	"testing"

	"github.com/orktes/orlang/ast"

	"github.com/orktes/orlang/parser"
)

func TestVisitor(t *testing.T) {
	file, err := parser.Parse(strings.NewReader(`
    fn foobar(x : int32, y : float32) : float32 {
      return x + y
    }

    fn main() {
      var foo = foobar(1, 0.32)
    }
  `))
	if err != nil {
		t.Error(err)
	}

	visitor := &visitor{
		scope: NewScope(),
		node:  file,
		info:  &FileInfo{},
	}

	ast.Walk(visitor, file)
}

package analyser

import (
	"strings"
	"testing"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/parser"
)

func TestClosureAnalysis(t *testing.T) {
	file, err := parser.Parse(strings.NewReader(`
    var global = 1
    fn main() {
      var a = 0
      var b = 1
      var c = 2
      fn foo() {
        a = 1
        fn bar() {
          b = 1
          fn fiz() {
            c = 4
            global++
          }
        }
      }
    }
	`))

	if err != nil {
		t.Error(err)
	}

	result, err := Analyse(file)
	if err != nil {
		t.Error(err)
	}

	closures := result.FileInfo[file].Closures

	check := func(closure *Closure, identifiers []string) {
		if len(closure.Env) != len(identifiers) {
			env := []string{}
			for _, scopeItem := range closure.Env {
				if varDecl, ok := scopeItem.(*ast.VariableDeclaration); ok {
					env = append(env, varDecl.Name.Text)
				}
			}
			t.Error("Wrong number of expected captured idents", env, identifiers)
		}

	envLoop:
		for _, scopeItem := range closure.Env {
			if varDecl, ok := scopeItem.(*ast.VariableDeclaration); ok {
				for _, ident := range identifiers {
					if varDecl.Name.Text == ident {
						continue envLoop
					}
				}
				t.Error("Could not find scope item")
			} else {
				t.Error("Not a var decl")
			}
		}
	}

	check(closures[0], []string{"a", "b", "c"})
	check(closures[1], []string{"b", "c"})
	check(closures[2], []string{"c"})
}

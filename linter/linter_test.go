package linter

import (
	"reflect"
	"strings"
	"testing"

	"github.com/orktes/orlang/ast"
)

func TestLinter(t *testing.T) {
	lintErrors, err := Lint(strings.NewReader(`
    fn main() {
      for var i = 1; i<; 1 {
        fn foobar() {
          for var i = 0; i; i++1 {
          }
        }
      }
    }
  `))

	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(lintErrors, []LintIssue{
		LintIssue{
			Position: ast.Position{Line: 2, Column: 23},
			Message:  "Expected expression got SEMICOLON(;)",
			CodeLine: "      for var i = 1; i<; 1 {",
		},
		LintIssue{
			Position: ast.Position{Line: 4, Column: 31},
			Message:  "Expected code block got NUMBER(1)",
			CodeLine: "          for var i = 0; i; i++1 {",
		},
	}) {
		t.Errorf("Output didnt match expected output %+v", lintErrors)
	}
}
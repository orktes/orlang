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
  `), nil)

	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(lintErrors, []LintIssue{
		LintIssue{
			Position:    ast.Position{Line: 2, Column: 23},
			EndPosition: ast.Position{Line: 2, Column: 24},
			Message:     "Expected expression got SEMICOLON(;)",
			CodeLine:    "",
		},
		LintIssue{
			Position:    ast.Position{Line: 4, Column: 31},
			EndPosition: ast.Position{Line: 4, Column: 32},
			Message:     "Expected code block got NUMBER(1)",
			CodeLine:    "",
		},
	}) {
		t.Errorf("Output didnt match expected output %+v", lintErrors)
	}
}

func TestLinterAnalyzeErrors(t *testing.T) {
	lintErrors, err := Lint(strings.NewReader(`
    fn main() {
			var foo = 0
			foo = 0.0
			foo = "foo"
    }
  `), nil)

	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(lintErrors, []LintIssue{
		LintIssue{
			Position:    ast.Position{Line: 3, Column: 9},
			EndPosition: lintErrors[0].EndPosition,
			Message:     "cannot use 0.0 (type float32) as type int32 in assigment expression",
			CodeLine:    "",
		},
		LintIssue{
			Position:    ast.Position{Line: 4, Column: 9},
			EndPosition: lintErrors[1].EndPosition,
			Message:     "cannot use \"foo\" (type string) as type int32 in assigment expression",
			CodeLine:    "",
		},
	}) {
		t.Errorf("Output didnt match expected output %+v", lintErrors)
	}
}

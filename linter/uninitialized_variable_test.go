package linter

import (
	"reflect"
	"strings"
	"testing"

	"github.com/orktes/orlang/ast"
)

func TestUninitializedVariableWarning(t *testing.T) {
	lintErrors, err := Lint(strings.NewReader(`
    fn main() {
      var i : int32
      var x = i
      i = 1
      var y = i
    }
  `), nil)

	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(lintErrors, []LintIssue{
		LintIssue{
			Position:    ast.Position{Line: 3, Column: 14},
			EndPosition: ast.Position{Line: 3, Column: 15},
			Message:     "variable i used before initialized",
			CodeLine:    "",
			Warning:     true,
		},
		LintIssue{
			Position:    ast.Position{Line: 3, Column: 10},
			EndPosition: ast.Position{Line: 3, Column: 11},
			Message:     "x declared but not used",
			CodeLine:    "",
			Warning:     true,
		},
		LintIssue{
			Position:    ast.Position{Line: 5, Column: 10},
			EndPosition: ast.Position{Line: 5, Column: 11},
			Message:     "y declared but not used",
			CodeLine:    "",
			Warning:     true,
		},
	}) {
		t.Errorf("Output didnt match expected output %+v", lintErrors)
	}
}

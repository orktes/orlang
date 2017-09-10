package analyser

import (
	"strings"
	"testing"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"

	"github.com/orktes/orlang/parser"
)

func TestAutoComplete(t *testing.T) {
	tests := []struct {
		src    string
		result []string
	}{
		{
			`
        var foo = 1

        fn main() {
          #
        }
      `,
			[]string{"foo", "main"},
		},
		{
			`
        var foo = 1
        var bar = 2

        fn main() {
          fo#
        }
      `,
			[]string{"foo"},
		},
		{
			`
        struct Foo {
          var bar = 1
        }
        var foo = Foo{}

        fn main() {
          foo.#
        }
      `,
			[]string{"bar"},
		},
		{
			`
        struct Foo {
          var bar = 1
        }

        fn main() {
          var foo = Fo#
        }
      `,
			[]string{"Foo"},
		},
		{
			`
        fn main() {
          var foo : int#
        }
      `,
			[]string{"int32", "int64"},
		},
		{
			`
        struct Foo {}
        fn main() {
          var foo : F#
        }
      `,
			[]string{"Foo"},
		},
	}

	for _, test := range tests {
		src := test.src
		var pos ast.Position

		lines := strings.Split(src, "\n")

	lineLoop:
		for lineNumber, line := range lines {
			if strings.Contains(line, "#") {
				pos.Line = lineNumber
				pos.Column = strings.Index(line, "#")
				break lineLoop
			}
		}

		src = strings.Replace(strings.Join(lines, "\n"), "#", "", -1)

		scanner := scanner.NewScanner(strings.NewReader(src))
		pars := parser.NewParser(NewAutoCompleteScanner(scanner, []ast.Position{pos}))

		file, err := pars.Parse()
		if err != nil {
			t.Error(err)
		}

		var result []AutoCompleteInfo
		visitor := &visitor{
			scope: NewScope(file),
			node:  file,
			types: map[string]ast.Node{},
			info:  NewFileInfo(),
			autocompleteCb: func(res []AutoCompleteInfo) {
				result = res
			},
			errorCb: func(node ast.Node, msg string, _ bool) {
			},
		}

		ast.Walk(visitor, file)

	keycheck:
		for _, keya := range test.result {
			for _, keyb := range result {
				if keya == keyb.Label {
					continue keycheck
				}
			}
			t.Errorf("Expected %s to return error %s, but got %s", test.src, test.result, result)
		}

	}
}

package parser

import (
	"strings"
	"testing"

	"github.com/orktes/orlang/ast"
)

func TestParseComments(t *testing.T) {
	file, err := Parse(strings.NewReader(`
		// This wont be attached to a anything

		// attached to foo
		fn foo() {}

		/* attached to bar */
		fn bar(
			/*attached to arg before: foo*/ foo: int /*attached to arg after: foo*/
			,
			/*attached to arg before: bar*/
			bar: int /*attached to arg after: bar*/
			,
			/*attached to arg before: buz*/
			baz: int
			/*Not attached to anything*/,
			) {

			// Testing
			var foo : bar = 1

		}

		var (
			foo : Foo, // attched to var foo
			// attached to var bar
			bar : Bar,
			baz: Baz,
			// not attched to anything
		)
  `))
	if err != nil {
		t.Error(err)
	}

	if len(file.Comments) != 3 {
		t.Error("Wrong number of comment attached", file.Comments)
	}

	if file.NodeComments[file.Body[0]][0].Token.Text != "// attached to foo" {
		t.Error("Wrong comment attached to node")
	}

	if file.NodeComments[file.Body[1]][0].Token.Text != "/* attached to bar */" {
		t.Error("Wrong comment attached to node", file.NodeComments[file.Body[1]][0])

	}

	argComments := file.NodeComments[file.Body[1].(*ast.FunctionDeclaration).Signature.Arguments[0]]
	if len(argComments) != 2 {
		t.Error("Wrong number of comments")
	}

	argComments = file.NodeComments[file.Body[1].(*ast.FunctionDeclaration).Signature.Arguments[1]]
	if len(argComments) != 2 {
		t.Error("Wrong number of comments")
	}

	varComments := file.NodeComments[file.Body[1].(*ast.FunctionDeclaration).Block.Body[0]]
	if varComments[0].Token.Text != "// Testing" {
		t.Error("Wrong comment")
	}

}

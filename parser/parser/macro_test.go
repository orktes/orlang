package parser

import (
	"fmt"
	"strings"
	"testing"

	"github.com/orktes/orlang/parser/ast"
	"github.com/orktes/orlang/parser/scanner"
)

func TestBlockInMacro(t *testing.T) {
	_, err := Parse(strings.NewReader(`
		macro ifMacro {
		  ($a:expr, $b:block) : (if $a $b)
		}
		fn main() {
			ifMacro!(true, {

			})
		}
	`))

	if err != nil {
		t.Error(err)
	}
}

func TestStatementInMacro(t *testing.T) {
	_, err := Parse(strings.NewReader(`
		macro stmt {
		  ($a:stmt) : ($a)
		}
		fn main() {
			stmt!(var foo : int = 0)
		}
	`))

	if err != nil {
		t.Error(err)
	}
}

func TestSimpleMacroRepetition(t *testing.T) {
	file, err := Parse(strings.NewReader(`
		macro test {
		  ($( $x:expr ),*): (
		    $(
		      $x
		    )*
		    $(
		      $x
		    )*
		  )
		}

		fn main() {
		  test!(1, 2, 3, 4, 5)
		}
  `))
	if err != nil {
		t.Error(err)
	}

	getValue := func(index int) int64 {
		return file.Body[1].(*ast.FunctionDeclaration).Block.Body[index].(*ast.ValueExpression).Value.(int64)
	}

	if getValue(0) != int64(1) {
		t.Error("Wrong value")
	}
	if getValue(1) != int64(2) {
		t.Error("Wrong value")
	}
	if getValue(2) != int64(3) {
		t.Error("Wrong value")
	}
	if getValue(3) != int64(4) {
		t.Error("Wrong value")
	}
	if getValue(4) != int64(5) {
		t.Error("Wrong value")
	}
	// Next iterator. Lets check only the first one
	if getValue(5) != int64(1) {
		t.Error("Wrong value")
	}
}

func TestMultipleMacroRepetition(t *testing.T) {
	file, err := Parse(strings.NewReader(`
		macro test {
		  (
		    $( foo ),*
		    $( bar ),*
		  ): (
		    $(
		       1
		    )*
		    $(
		       2
		    )*
		  )
		}

		fn main() {
		  test!(foo, foo, bar)
		}
  `))
	if err != nil {
		t.Error(err)
	}

	getValue := func(index int) int64 {
		return file.Body[1].(*ast.FunctionDeclaration).Block.Body[index].(*ast.ValueExpression).Value.(int64)
	}

	if getValue(0) != int64(1) {
		t.Error("Wrong value")
	}
	if getValue(1) != int64(1) {
		t.Error("Wrong value")
	}
	if getValue(2) != int64(2) {
		t.Error("Wrong value")
	}
}

func TestMacroBrainFuck(t *testing.T) {
	t.Skip()

	file, err := Parse(strings.NewReader(`
		macro bf {
		  (
		    $(
		      $( > )?
		      $( < )?
		      $( + )?
		      $( ++ )?
		      $( - )?
		      $( -- )?
		      $( . )?
		      $( , )?
		      $( [ )?
		      $( ] )?
		    )*
		  ): (
		    fn (input: []char) {
		      var data : []char; // TODO initialize
		      var dataPointer : int = 0
		      var i : int = 0;
		      $(
		        $(
		          dataPointer++
		        )*
		        $(
		          dataPointer--
		        )*
		        $(
		          data[dataPointer] = data[dataPointer] + 1
		        )*
		        $(
		          data[dataPointer] = data[dataPointer] + 2
		        )*
		        $(
		          data[dataPointer] = data[dataPointer] - 1
		        )*
		        $(
		          data[dataPointer] = data[dataPointer] - 2
		        )*
		        $(
		          putchar(data[dataPointer])
		        )*
		        $(
		          data[dataPointer] = getchar()
		        )*
		        $(
		          // Just continue
		        )
		        $(
		          loop = 1;
		          for loop > 0 {
		            var current_char = input[--i];
		            if (current_char == '[') {
		              loop--;
		            } else if (current_char == ']') {
		              loop++;
		            }
		          }
		        )*
		      )*
		    }
		  )
		}

		fn main() {
		  bf!(
		    ,[.[-],]
		  )(input: input) // TODO array input
		}

  `))
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%#v\n", file)
}

func TestMacro(t *testing.T) {
	file, err := Parse(strings.NewReader(`
		macro fooMacro {
			{$a:block}: {if true $a}
			($a:expr, $b:expr) : ($a + $b)
			($a:expr {} $b:expr) : ($a - $b)
			($a:expr { $b:expr }) : ($a * $b)
			($a:expr {}) : (fooMacro!($a, 1))
			() : (1)
		}
		fn main() {
			var foo = fooMacro!(1,2)
			var bar = fooMacro![1 {} ]
			var bar = fooMacro!{1 {} 2}
			var bar = fooMacro!(1 { 2 })
			var bar = fooMacro!
			fooMacro!({

			})
		}
	`))
	if err != nil {
		t.Error(err)
	}

	macro := file.Body[0].(*ast.Macro)
	pattern := macro.Patterns[1]

	if pattern.Pattern[0].(*ast.MacroMatchArgument).Name != "$a" {
		t.Error("Wrong pattern key name")
	}

	if pattern.Pattern[0].(*ast.MacroMatchArgument).Type != "expr" {
		t.Error("Wrong pattern key type")
	}

	if pattern.Pattern[2].(*ast.MacroMatchArgument).Name != "$b" {
		t.Error("Wrong pattern key name")
	}

	if pattern.Pattern[2].(*ast.MacroMatchArgument).Type != "expr" {
		t.Error("Wrong pattern key type")
	}

	tokens := pattern.TokensSets[0].(ast.MacroTokenSliceSet)
	if len(tokens) != 3 {
		t.Error("Wrong amount of tokens", tokens)
	}

	if tokens[0].StringValue() != "MACROIDENT($a)" {
		t.Error("Wrong token")
	}

	if tokens[1].StringValue() != "ADD(+)" {
		t.Error("Wrong token")
	}

	if tokens[2].StringValue() != "MACROIDENT($b)" {
		t.Error("Wrong token")
	}

	binaryExpr := file.Body[1].(*ast.FunctionDeclaration).Block.Body[0].(*ast.VariableDeclaration).DefaultValue.(*ast.BinaryExpression)
	if binaryExpr.Left.(*ast.ValueExpression).Value != int64(1) {
		t.Error("Wrong value")
	}

	if binaryExpr.Right.(*ast.ValueExpression).Value != int64(2) {
		t.Error("Wrong value")
	}

	binaryExpr = file.Body[1].(*ast.FunctionDeclaration).Block.Body[1].(*ast.VariableDeclaration).DefaultValue.(*ast.BinaryExpression)
	if binaryExpr.Left.(*ast.ValueExpression).Value != int64(1) {
		t.Error("Wrong value")
	}

	if binaryExpr.Right.(*ast.ValueExpression).Value != int64(1) {
		t.Error("Wrong value")
	}

	binaryExpr = file.Body[1].(*ast.FunctionDeclaration).Block.Body[2].(*ast.VariableDeclaration).DefaultValue.(*ast.BinaryExpression)
	if binaryExpr.Left.(*ast.ValueExpression).Value != int64(1) {
		t.Error("Wrong value")
	}

	if binaryExpr.Right.(*ast.ValueExpression).Value != int64(2) {
		t.Error("Wrong value")
	}

	if binaryExpr.Operator.Type != scanner.TokenTypeSUB {
		t.Error("Wrong operator")
	}

	binaryExpr = file.Body[1].(*ast.FunctionDeclaration).Block.Body[3].(*ast.VariableDeclaration).DefaultValue.(*ast.BinaryExpression)
	if binaryExpr.Left.(*ast.ValueExpression).Value != int64(1) {
		t.Error("Wrong value")
	}

	if binaryExpr.Right.(*ast.ValueExpression).Value != int64(2) {
		t.Error("Wrong value")
	}

	if binaryExpr.Operator.Type != scanner.TokenTypeASTERISK {
		t.Error("Wrong operator")
	}

	if file.Body[1].(*ast.FunctionDeclaration).Block.Body[4].(*ast.VariableDeclaration).DefaultValue.(*ast.ValueExpression).Value != int64(1) {
		t.Error("Wrong value")
	}

}

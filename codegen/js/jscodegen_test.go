package js

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/types"

	"github.com/orktes/orlang/analyser"
	"github.com/orktes/orlang/parser"
	"github.com/robertkrimen/otto"
)

func TestSimple(t *testing.T) {
	res, err := testCodegen(`
    macro createTuple {
      ($a:expr , $( $x:expr ),*) : (
        (
          $a
          $(
            ,
            $x
          )*
        )
      )
    }

    fn getData() : (int32, int32) {
      return createTuple!(1, 2)
    }

    fn sum(a : float64, b : float64) : int32 {
      return int32(a + b)
    }

    fn main() {
      var ab = getData()

      var (a, b) : (int32, int32) = ab
      var abSum = sum(
        b: float64(b),
        a: float64(a)
      )

      var negative = int64(-((1 + 4) * int32(5.5)))
      var (x, y, (h, j)) = (1, 2, ab)

      print(
        "result is: " +
        int_to_str(int64(abSum - int32(1.5))) +
        " and " +
        int_to_str(int64(h + j)) +
        " and " +
        int_to_str(negative)
      )
    }
  `)

	if err != nil {
		t.Fatal(err)
	}

	if res != "result is: 2 and 3 and -25" {
		t.Error("Wrong result received", res)
	}
}

func testCodegen(str string) (string, error) {
	file, err := parser.Parse(strings.NewReader(str))
	if err != nil {
		return "", err
	}

	analyser, err := analyser.New(file)
	if err != nil {
		return "", err
	}

	analyser.AddExternalFunc("int_to_str", &types.SignatureType{
		ArgumentNames: []string{"num"},
		ArgumentTypes: []types.Type{types.Int64Type},
		ReturnType:    types.StringType,
	})

	analyser.AddExternalFunc("print", &types.SignatureType{
		ArgumentNames: []string{"str"},
		ArgumentTypes: []types.Type{types.StringType},
		ReturnType:    types.VoidType,
	})

	analyser.AddExternalFunc("printInt", &types.SignatureType{
		ArgumentNames: []string{"num"},
		ArgumentTypes: []types.Type{types.Int64Type},
		ReturnType:    types.VoidType,
	})

	var analyErr error
	analyser.Error = func(node ast.Node, msg string, fatal bool) {
		if fatal {
			analyErr = errors.New(msg)
		}
	}

	info, err := analyser.Analyse()
	if err != nil {
		return "", err
	}

	if analyErr != nil {
		return "", analyErr
	}

	code := New(info).Generate(file)
	println(string(code)) // debug

	var result string
	vm := otto.New()
	vm.Set("print", func(call otto.FunctionCall) otto.Value {
		result = call.Argument(0).String()
		return otto.Value{}
	})

	vm.Set("int_to_str", func(call otto.FunctionCall) otto.Value {
		result = call.Argument(0).String()
		val, _ := vm.ToValue(result)
		return val
	})

	vm.Set("printInt", func(call otto.FunctionCall) otto.Value {
		resultInt, _ := call.Argument(0).ToInteger()
		result = fmt.Sprintf("%d", resultInt)
		return otto.Value{}
	})
	_, err = vm.Run(string(code))

	return result, err
}

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

    fn getData() => (int32, int32) {
      return createTuple!(1, 2)
    }

    fn sum(a : float64, b : float64 = float64(100.0)) => int32 {
      return int32(a + b)
    }

		fn callback(cb : ((int32) => void) = fn (a : int32) {}) {
			cb(1)
		}

    var ab = getData()

    fn main() {

      var (a, b) : (int32, int32) = ab
      var abSum = sum(
        b: float64(b),
        a: float64(a)
      )

      var negative = int64(-((1 + 4) * int32(5.5)))
      var (x, y, (h, j)) = (1, 2, ab)

      var counter = 0

      for counter < 10 {
				var a = 100 // This should not affect result
        counter++
      }

			var sum100 = sum(a: float64(1))

			callback(fn (a : int32) {
				sum100 = sum100 + a
			})

      if true {
        print(
          "result is: " +
          int_to_str(int64(abSum - int32(1.5))) +
          " and " +
          int_to_str(int64(h + j)) +
          " and " +
          int_to_str(negative) +
          " and " +
          int_to_str(int64(counter)) +
					" and " +
          int_to_str(int64(sum100))
        )
      } else if false {
        print("Will not ever be here")
      } else {
				print("New else")
			}
    }
  `)

	if err != nil {
		t.Fatal(err)
	}

	if res != "result is: 2 and 3 and -25 and 10 and 102" {
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
		Extern:        true,
	})

	analyser.AddExternalFunc("print", &types.SignatureType{
		ArgumentNames: []string{"str"},
		ArgumentTypes: []types.Type{types.StringType},
		ReturnType:    types.VoidType,
		Extern:        true,
	})

	analyser.AddExternalFunc("printInt", &types.SignatureType{
		ArgumentNames: []string{"num"},
		ArgumentTypes: []types.Type{types.Int64Type},
		ReturnType:    types.VoidType,
		Extern:        true,
	})

	var analyErr error
	analyser.Error = func(node ast.Node, msg string, fatal bool) {
		if fatal {
			errStr := fmt.Sprintf(
				"%d:%d %s",
				node.StartPos().Line+1,
				node.StartPos().Column+1,
				msg,
			)
			analyErr = errors.New(errStr)
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

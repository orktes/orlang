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
    fn sum(a : float64, b : float64) : float64 {
      return a + b
    }

    fn main() {
      var a = 1
      var b = 2

      var abSum = int32(
        sum(
          float64(a),
          float64(b)
        )
      )

      print("result is: " + int_to_str(int64(abSum - int32(1.5))))
    }
  `)

	if err != nil {
		t.Fatal(err)
	}

	if res != "result is: 2" {
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
	// println(string(code)) // debug

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

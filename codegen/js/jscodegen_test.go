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
		macro stringCat {
			() : (
				fn +(left:string, right:float32) => string {
					return left + num_to_str(float64(right))
				}

				fn +(left:string, right:float64) => string {
					return left + num_to_str(float64(right))
				}

				fn +(left:string, right:int32) => string {
					return left + num_to_str(float64(right))
				}

				fn +(left:string, right:int64) => string {
					return left + num_to_str(float64(right))
				}

				fn +(left:int32, right:string) => string {
					return num_to_str(float64(left)) + right
				}

				fn +(left:int64, right:string) => string {
					return num_to_str(float64(left)) + right
				}

				fn +(left:float32, right:string) => string {
					return num_to_str(float64(left)) + right
				}

				fn +(left:float64, right:string) => string {
					return num_to_str(float64(left)) + right
				}
			)
		}


		macro print {
			($a:expr) : (
				(fn () => void {
					stringCat!()
					print($a)
				})()
			)

			($a:expr, $( $x:expr ),*) : (
				print!(
					$a
          $(
						+
						" "
            +
            $x
          )*
				)
			)
		}

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

		struct CustomStruct {
			var foo = 1
			var bar = 0

			fn +(left:CustomStruct, right:CustomStruct) => int32 {
				return left.foo + right.foo
			}

			fn foobar(a: int32) => int32 {
				return this.foo * a
			}

			fn incrementBar() {
				this.bar = this.bar + 1
			}
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

			fn +(left:int32, right:int32) => int32 {
				return left - right
			}

			fn -(left:int32, right:float32) => int32 {
				return left - int32(right)
			}

			var overloaded = (10 + 9) - 1.0

			var structSum = CustomStruct{} + CustomStruct{}
			var foobarValue = (CustomStruct{}).foobar(100)

			var structVal = CustomStruct{}
			structVal.incrementBar()
			structVal.incrementBar()
			structVal.incrementBar()
			structVal.incrementBar()

      if true {
        print!(
          "result is:",
          abSum - 1.5,
          "and",
          h + j,
          "and",
        	negative,
          "and",
          counter,
					"and",
          sum100,
					"and",
          overloaded,
					"and",
					structSum,
					"and",
					foobarValue,
					"and",
					structVal.bar
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

	if res != "result is: 2 and -1 and -25 and 10 and 102 and 0 and 2 and 100 and 4" {
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

	analyser.AddExternalFunc("num_to_str", &types.SignatureType{
		ArgumentNames: []string{"num"},
		ArgumentTypes: []types.Type{types.Float64Type},
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

	vm.Set("num_to_str", func(call otto.FunctionCall) otto.Value {
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

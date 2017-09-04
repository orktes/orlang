package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/orktes/orlang/ast"

	"github.com/gopherjs/gopherjs/js"
	"github.com/orktes/orlang/analyser"
	jscodegen "github.com/orktes/orlang/codegen/js"
	"github.com/orktes/orlang/parser"
	"github.com/orktes/orlang/types"
)

func main() {
	js.Global.Set("Orlang", map[string]interface{}{
		"Compile": func(input string) string {
			file, err := parser.Parse(strings.NewReader(input))
			if err != nil {
				panic(err)
			}

			analyser, err := analyser.New(file)
			if err != nil {
				panic(err)
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
				panic(err)
			}

			if analyErr != nil {
				panic(err)
			}

			code := jscodegen.New(info).Generate(file)

			return string(code)
		},
	})
}

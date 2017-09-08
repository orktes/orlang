package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/linter"
	"github.com/orktes/orlang/scanner"

	"github.com/gopherjs/gopherjs/js"
	"github.com/orktes/orlang/analyser"
	jscodegen "github.com/orktes/orlang/codegen/js"
	"github.com/orktes/orlang/parser"
	"github.com/orktes/orlang/types"
)

func configureAnalyzer(analsr *analyser.Analyser) {
	analsr.AddExternalFunc("int_to_str", &types.SignatureType{
		ArgumentNames: []string{"num"},
		ArgumentTypes: []types.Type{types.Int64Type},
		ReturnType:    types.StringType,
		Extern:        true,
	})

	analsr.AddExternalFunc("print", &types.SignatureType{
		ArgumentNames: []string{"str"},
		ArgumentTypes: []types.Type{types.StringType},
		ReturnType:    types.VoidType,
		Extern:        true,
	})
}

func main() {

	js.Module.Get("exports").Set("Lint", func(input string) []linter.LintIssue {
		errors, err := linter.Lint(strings.NewReader(input), configureAnalyzer)
		if err != nil {
			panic(err)
		}

		return errors
	})

	js.Module.Get("exports").Set("Tokenize", func(input string) []scanner.Token {
		scan := scanner.NewScanner(strings.NewReader(input))

		tokens := []scanner.Token{}
		for {
			token := scan.Scan()
			tokens = append(tokens, token)
			if token.Type == scanner.TokenTypeEOF {
				break
			}
		}

		return tokens
	})

	js.Module.Get("exports").Set("Compile", func(input string) string {
		file, err := parser.Parse(strings.NewReader(input))
		if err != nil {
			panic(err)
		}

		analyser, err := analyser.New(file)
		if err != nil {
			panic(err)
		}

		configureAnalyzer(analyser)

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
	})
}

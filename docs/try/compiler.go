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
	analsr.AddExternalFunc("print", &types.SignatureType{
		ArgumentNames: []string{"str"},
		ArgumentTypes: []types.Type{&types.InterfaceType{
			Name: "buildin(stringer)",
			Functions: []struct {
				Name string
				Type *types.SignatureType
			}{
				{
					Name: "toString",
					Type: &types.SignatureType{
						ArgumentNames: []string{},
						ArgumentTypes: []types.Type{},
						ReturnType:    types.StringType,
						Extern:        true,
					},
				},
			},
		}},
		ReturnType: types.VoidType,
		Extern:     true,
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

	js.Module.Get("exports").Set("AutoComplete", func(input string, line int, column int) (items []map[string]interface{}) {
		scanner := scanner.NewScanner(strings.NewReader(input))
		pars := parser.NewParser(analyser.NewAutoCompleteScanner(scanner, []ast.Position{ast.Position{Line: line, Column: column}}))

		file, err := pars.Parse()
		if err != nil {
			panic(err)
		}

		var result []analyser.AutoCompleteInfo
		alys, err := analyser.New(file)
		if err != nil {
			panic(err)
		}

		configureAnalyzer(alys)

		alys.AutoCompleteInfoCallback = func(res []analyser.AutoCompleteInfo) {
			result = res
		}

		alys.Analyse()

	itemLoop:
		for _, item := range result {
			switch item.Label {
			case "+", "-", "*", "/":
				continue itemLoop
			}

			insertText := item.Label
			switch t := item.Type.(type) {
			case *types.SignatureType:
				args := make([]string, len(t.ArgumentNames))
				for i, arg := range t.ArgumentNames {
					args[i] = fmt.Sprintf("%s: ${%d:%s}", arg, i+1, arg)
				}
				insertText = fmt.Sprintf("%s(%s)", insertText, strings.Join(args, ", "))
			case *types.StructType:
				if item.Kind == "Class" {
					args := make([]string, len(t.Variables))
					for i, v := range t.Variables {
						args[i] = fmt.Sprintf("%s: ${%d:%s}", v.Name, i+1, v.Name)
					}
					insertText = fmt.Sprintf("%s{%s}", insertText, strings.Join(args, ", "))
				}
			}

			items = append(items, map[string]interface{}{
				"label":         item.Label,
				"insertText":    insertText,
				"kind":          item.Kind,
				"documentation": item.Type.GetName(),
				"detail":        item.Type.GetName(),
				"type":          fmt.Sprintf("%T", item.Type),
			})
		}

		for macroName, _ := range file.Macros {
			items = append(items, map[string]interface{}{
				"label":         macroName + "!",
				"insertText":    macroName + "!",
				"kind":          "Reference",
				"documentation": "macro " + macroName,
				"detail":        "macro " + macroName,
				"type":          "macro",
			})
		}

		return
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

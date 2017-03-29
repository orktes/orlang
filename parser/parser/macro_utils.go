package parser

import "github.com/orktes/orlang/parser/ast"

func macroPatternMatchesNextType(t string, pattern *ast.MacroPattern, values []interface{}) bool {
	indx := len(values)
	if len(pattern.Pattern) > indx {
		if m, ok := pattern.Pattern[indx].(*ast.MacroMatchArgument); ok {
			if m.Type == t {
				return true
			}
		}
	}
	return false
}

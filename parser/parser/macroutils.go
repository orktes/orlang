package parser

import (
	"github.com/orktes/orlang/parser/ast"
	"github.com/orktes/orlang/parser/scanner"
)

type macroProcessor struct {
	pattern       []ast.MacroMatch
	currentPos    int
	noMatch       bool
	subProcessors map[ast.MacroMatch]*macroProcessor
	values        map[string][]interface{}
	repeating     bool
	loops         int
}

func newMacroPreprocessor(pattern []ast.MacroMatch, repeating bool) (mp *macroProcessor) {
	mp = &macroProcessor{
		pattern:       pattern,
		subProcessors: map[ast.MacroMatch]*macroProcessor{},
		values:        map[string][]interface{}{},
		repeating:     repeating,
	}
	for _, mm := range pattern {
		if mmr, ok := mm.(*ast.MacroMatchRepetition); ok {
			mp.subProcessors[mm] = newMacroPreprocessor(mmr.Pattern, true)
		}
	}
	return
}

func (mp *macroProcessor) ok() bool {
	if mp.noMatch {
		return false
	}

	if mp.currentPos == len(mp.pattern) {
		return true
	}

	mm := mp.pattern[mp.currentPos]
	if subProcessor, ok := mp.subProcessors[mm]; ok {
		subProcessorOk := subProcessor.ok()
		if subProcessorOk {
			return true
		}

		if subProcessor.currentPos == 0 {
			if mmr, ok := mm.(*ast.MacroMatchRepetition); ok {
				if mmr.Operand.Type == scanner.TokenTypeADD {
					// + detonates that there needs to be at least one match
					if subProcessor.loops == 0 {
						return false
					}
				}

				return true
			}
		}
	}

	return false
}

func (mp *macroProcessor) acceptsType(t string) bool {
	return mp.acceptsTypeWithIndex(mp.currentPos, t)
}

func (mp *macroProcessor) feed(val interface{}) (accepts bool) {
	if mp.noMatch {
		return false
	}

	var mm ast.MacroMatch
	var indx int

feed:
	if mp.currentPos > len(mp.pattern)-1 {
		if !mp.repeating {
			goto result
		}
		mp.currentPos = 0
	}

	indx = mp.currentPos
	mm = mp.pattern[indx]
	if subProcessor, ok := mp.subProcessors[mm]; ok {
		noMatchState := subProcessor.noMatch
		currentPosState := subProcessor.currentPos
		loopsState := subProcessor.loops

		accepts = subProcessor.feed(val)
		if accepts {
			goto result
		} else {
			subProcessor.noMatch = noMatchState
			subProcessor.currentPos = currentPosState
			subProcessor.loops = loopsState
		}

		if mmr, ok := mm.(*ast.MacroMatchRepetition); ok {
			if mmr.Operand.Type == scanner.TokenTypeADD {
				// + detonates that there needs to be at least one match
				if subProcessor.loops == 0 {
					goto result
				}
			}

			if subProcessor.ok() {
				mp.currentPos++
				goto feed
			} else {
				goto result
			}
		}
	}

	switch m := mm.(type) {
	case *ast.MacroMatchArgument:
		switch m.Type {
		case "block":
			_, accepts = val.(*ast.Block)
		case "expr":
			_, accepts = val.(ast.Expression)
		case "stmt":
			_, accepts = val.(ast.Statement)
		}

		if accepts {
			mp.values[m.Name] = append(mp.values[m.Name], val)
		}
	case *ast.MacroMatchToken:
		var t scanner.Token
		if t, accepts = val.(scanner.Token); accepts {
			// TODO create a better way to compare tokens
			accepts = t.StringValue() == m.Token.StringValue()
		}
	}

	if accepts {
		mp.currentPos++
		if mp.currentPos > len(mp.pattern)-1 {
			mp.loops++
		}
	}

result:
	if !accepts {
		mp.noMatch = true
	}

	return
}

func (mp *macroProcessor) acceptsTypeWithIndex(indx int, t string) (accepts bool) {
	if mp.noMatch {
		return false
	}
	if indx > len(mp.pattern)-1 {
		return false
	}
	mm := mp.pattern[indx]
	if subProcessor, ok := mp.subProcessors[mm]; ok {
		accepts = subProcessor.acceptsType(t)
		if accepts {
			return true
		}

		if mmr, ok := mm.(*ast.MacroMatchRepetition); ok {
			if mmr.Operand.Type == scanner.TokenTypeADD {
				// + detonates that there needs to be at least one match
				if subProcessor.loops == 0 {
					return false
				}
			}
			return mp.acceptsTypeWithIndex(indx+1, t)
		}
	}

	switch val := mm.(type) {
	case *ast.MacroMatchArgument:
		accepts = val.Type == t
	case *ast.MacroMatchToken:
		accepts = t == "token" // Figure out some better way to check this
	}

	return
}

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

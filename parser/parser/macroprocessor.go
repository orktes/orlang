package parser

import (
	"fmt"

	"github.com/orktes/orlang/parser/ast"
	"github.com/orktes/orlang/parser/scanner"
)

type macroProcessor struct {
	pattern              []ast.MacroMatch
	currentPos           int
	noMatch              bool
	subProcessors        map[ast.MacroMatch]*macroProcessor
	orderedSubProcessors []*macroProcessor
	parentProcessor      *macroProcessor
	values               map[string][]interface{}
	repeating            bool
	delimiter            *scanner.Token
	delimiterSet         bool
	loops                int
}

func newMacroPreprocessor(pattern []ast.MacroMatch, repeating bool) (mp *macroProcessor) {
	mp = &macroProcessor{
		pattern:              pattern,
		subProcessors:        map[ast.MacroMatch]*macroProcessor{},
		orderedSubProcessors: []*macroProcessor{},
		values:               map[string][]interface{}{},
		repeating:            repeating,
	}
	for _, mm := range pattern {
		if mmr, ok := mm.(*ast.MacroMatchRepetition); ok {
			sp := newMacroPreprocessor(mmr.Pattern, true)
			sp.parentProcessor = mp
			sp.delimiter = mmr.Delimiter
			mp.subProcessors[mm] = sp
			mp.orderedSubProcessors = append(mp.orderedSubProcessors, sp)
		}
	}
	return
}

func (mp *macroProcessor) get(key string, index int) interface{} {
	vals, ok := mp.values[key]
	if !ok {
		if mp.parentProcessor != nil {
			return mp.parentProcessor.get(key, index)
		}
		return nil
	}

	valLen := len(vals)
	if index > valLen-1 {
		index = valLen - 1
	}

	return vals[index]
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

func (mp *macroProcessor) waitingForDelimiter() (tok scanner.Token, waiting bool) {
	if mp.repeating && mp.delimiter != nil {
		return *mp.delimiter, mp.loops > 0 && mp.currentPos == len(mp.pattern) && !mp.delimiterSet
	}
	return
}

func (mp *macroProcessor) feed(val interface{}) (accepts bool) {
	if mp.noMatch {
		return false
	}
	var mm ast.MacroMatch
	var indx int

	if del, waiting := mp.waitingForDelimiter(); waiting {
		var t scanner.Token
		if t, accepts = val.(scanner.Token); accepts {
			// TODO create a better way to compare tokens
			accepts = t.StringValue() == del.StringValue()
			if accepts {
				mp.delimiterSet = true
			}
		}
		goto result
	}

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
			if mmr, ok := mm.(*ast.MacroMatchRepetition); ok {
				// Should be iterated at most once
				if mmr.Operand.Type == scanner.TokenTypeQUESTIONMARK {
					goto cursorStep
				}
			}

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
		case "token":
			_, accepts = val.(scanner.Token)
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

cursorStep:
	if accepts {
		mp.delimiterSet = false
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

	if _, waiting := mp.waitingForDelimiter(); waiting {
		// subProcessor is waiting for delimiter
		return t == "token"
	}

	if indx > len(mp.pattern)-1 {
		if !mp.repeating {
			return false
		}
		indx = 0
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

func (mp *macroProcessor) expand(sets []ast.MacroTokenSet) (tokens []scanner.Token, err error) {
	reps := 0
	for _, set := range sets {
		switch s := set.(type) {
		case ast.MacroRepetitionTokenSet:
			if reps > len(mp.orderedSubProcessors)-1 {
				reps = len(mp.orderedSubProcessors) - 1
			}
			sp := mp.orderedSubProcessors[reps]
			reps++
			var newTokens []scanner.Token
			newTokens, err = sp.expand(s.Sets)
			if err != nil {
				return
			}
			tokens = append(tokens, newTokens...)
		case ast.MacroTokenSliceSet:
			newTokens := make([]scanner.Token, len(s))
			if mp.loops == 0 {
				// Macro had no call arguments. i.e matcher has not looped
				mp.loops = 1
			}
			for i := 0; i < mp.loops; i++ {
				for index, token := range s {
					if token.Type == scanner.TokenTypeMacroIdent {
						val := mp.get(token.Text, i)
						if val == nil {
							err = fmt.Errorf("Could not find macro argument for metavariable %s", token.Text)
							return
						}
						token.Value = val
					}
					newTokens[index] = token
				}
				tokens = append(tokens, newTokens...)
			}
		}
	}

	return
}

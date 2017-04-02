package parser

import "github.com/orktes/orlang/parser/ast"

type macroMatcherProcessor struct {
	pattern   *ast.MacroPattern
	processor *macroProcessor
}

type macroMatcher struct {
	macro      *ast.Macro
	processors []macroMatcherProcessor
}

func newMacroMatcher(macro *ast.Macro) (mm *macroMatcher) {
	processors := make([]macroMatcherProcessor, len(macro.Patterns))
	for i, mp := range macro.Patterns {
		processors[i] = macroMatcherProcessor{
			processor: newMacroPreprocessor(mp.Pattern, false),
			pattern:   mp,
		}
	}

	mm = &macroMatcher{
		processors: processors,
		macro:      macro,
	}
	return
}

func (mm *macroMatcher) acceptsType(t string) bool {
	for _, mm := range mm.processors {
		if mm.processor.acceptsType(t) {
			return true
		}
	}

	return false
}

func (mm *macroMatcher) feed(val interface{}) {
	for i := 0; i < len(mm.processors); i++ {
		p := mm.processors[i]
		if !p.processor.feed(val) {
			mm.processors = append(mm.processors[:i], mm.processors[i+1:]...)
			i--
		}
	}
}

func (mm *macroMatcher) match() *macroMatcherProcessor {
	for _, p := range mm.processors {
		if p.processor.ok() {
			return &p
		}
	}
	return nil
}

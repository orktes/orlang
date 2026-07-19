package analyser

import (
	"os"
	"fmt"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
	"github.com/orktes/orlang/types"
)

type Analyser struct {
	main                     *ast.File
	scope                    *Scope
	Error                    func(node ast.Node, msg string, fatal bool)
	AutoCompleteInfoCallback func([]AutoCompleteInfo)
	FileLoader               func(path string) (*ast.File, error)
}

func New(file *ast.File) (analyser *Analyser, err error) {
	analyser = &Analyser{
		main:  file,
		scope: NewScope(file),
	}
	return
}

func (analyser *Analyser) AddExternalFunc(name string, typ types.Type) {
	ident := &ast.Identifier{Token: scanner.Token{Text: name}}
	scopeItem := &CustomTypeResolvingScopeItem{
		ResolvedType: typ,
	}
	analyser.scope.Set(ident, scopeItem)

	// Hack to fix unused variale lint errors for build-in funcs
	// TODO figure out a proper way to do this
	analyser.scope.MarkUsage(scopeItem, &ast.Identifier{Token: scanner.Token{Text: name}})
}

func (analyser *Analyser) Analyse() (info *Info, err error) {
	// Analyser bugs must not crash the compiler process; surface them as
	// ordinary errors instead. (ORLANG_PANIC=1 re-panics for debugging.)
	defer func() {
		if r := recover(); r != nil {
			if os.Getenv("ORLANG_PANIC") == "1" {
				panic(r)
			}
			err = fmt.Errorf("internal analyser error: %v", r)
		}
	}()

	fileInfo := NewFileInfo()

	info = &Info{
		FileInfo: map[*ast.File]*FileInfo{},
	}

	info.FileInfo[analyser.main] = fileInfo

	visitor := &visitor{
		scope:          analyser.scope,
		node:           analyser.main,
		info:           fileInfo,
		autocompleteCb: analyser.AutoCompleteInfoCallback,
		errorCb: func(node ast.Node, err string, fatal bool) {
			if analyser.Error != nil {
				analyser.Error(node, err, fatal)
			}
		},
		fileLoader: analyser.FileLoader,
	}

	ast.Walk(visitor, analyser.main)
	return
}

func Analyse(file *ast.File) (info *Info, err error) {
	alrz, err := New(file)
	if err != nil {
		return nil, err
	}

	return alrz.Analyse()
}

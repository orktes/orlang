package analyser

import "github.com/orktes/orlang/ast"

type Analyser struct {
	main  *ast.File
	scope *Scope
	Error func(node ast.Node, msg string, fatal bool)
}

func New(file *ast.File) (analyser *Analyser, err error) {
	analyser = &Analyser{
		main:  file,
		scope: NewScope(file),
	}
	return
}

func (analyser *Analyser) Analyse() (info *Info, err error) {
	visitor := &visitor{
		scope: analyser.scope,
		node:  analyser.main,
		info:  &FileInfo{},
		errorCb: func(node ast.Node, err string, fatal bool) {
			if analyser.Error != nil {
				analyser.Error(node, err, fatal)
			}
		},
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

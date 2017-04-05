package analyzer

import "github.com/orktes/orlang/ast"

type Analyzer struct {
	main       *ast.File
	FileLoader func(importFile string) (file *ast.File, err error)
}

func NewAnalyzer(file *ast.File) (analyzer *Analyzer, err error) {
	analyzer = &Analyzer{
		main: file,
	}
	return
}

func (analyzer *Analyzer) Analyze() (info *Info) {
	return
}

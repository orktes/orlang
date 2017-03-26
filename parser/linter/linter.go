package linter

import (
	"io"

	"github.com/orktes/orlang/parser/ast"
	"github.com/orktes/orlang/parser/parser"
	"github.com/orktes/orlang/parser/scanner"
	"github.com/orktes/orlang/parser/util"
)

type LintIssue struct {
	Position ast.Position
	Message  string
	CodeLine string
	Warning  bool
}

func Lint(r io.Reader) (issues []LintIssue, err error) {
	hr := util.NewHistoryReader(r)
	p := parser.NewParser(scanner.NewScanner(hr))
	p.ContinueOnErrors = true
	lastTokenErrorIndex := -2
	p.Error = func(tokenIndx int, pos ast.Position, message string) {
		if tokenIndx != lastTokenErrorIndex+1 {
			line := hr.FindLineForPosition(pos)
			issues = append(issues, LintIssue{
				Position: pos,
				Message:  message,
				CodeLine: line,
				Warning:  false,
			})
		}
		lastTokenErrorIndex = tokenIndx
	}
	_, err = p.Parse()
	return
}

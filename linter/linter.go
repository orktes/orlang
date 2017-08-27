package linter

import (
	"io"

	"github.com/orktes/orlang/analyser"
	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/parser"
	"github.com/orktes/orlang/scanner"
	"github.com/orktes/orlang/util"
)

type LintIssue struct {
	Position    ast.Position
	EndPosition ast.Position
	Message     string
	CodeLine    string
	Warning     bool
}

func Lint(r io.Reader) (issues []LintIssue, err error) {
	hr := util.NewHistoryReader(r)
	p := parser.NewParser(scanner.NewScanner(hr))
	p.ContinueOnErrors = true
	lastTokenErrorIndex := -2
	p.Error = func(tokenIndx int, pos ast.Position, endPosition ast.Position, message string) {
		if tokenIndx != lastTokenErrorIndex+1 {
			line := hr.FindLineForPosition(pos)
			issues = append(issues, LintIssue{
				Position:    pos,
				EndPosition: endPosition,
				Message:     message,
				CodeLine:    line,
				Warning:     false,
			})
		}
		lastTokenErrorIndex = tokenIndx
	}

	file, err := p.Parse()
	if err != nil {
		return
	}

	analyser, err := analyser.New(file)
	if err != nil {
		return
	}

	analyser.Error = func(node ast.Node, message string, fatal bool) {
		issues = append(issues, LintIssue{
			Position:    node.StartPos(),
			EndPosition: node.EndPos(),
			Message:     message,
			CodeLine:    "", // TODO get line from file
			Warning:     !fatal,
		})
	}

	analyser.Analyse()

	return
}

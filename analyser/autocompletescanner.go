package analyser

import (
	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

type autoCompleteScanner struct {
	scanner      scanner.ScannerInterface
	cursors      []ast.Position
	markerPlaced bool
	nextToken    *scanner.Token
}

func NewAutoCompleteScanner(scanner scanner.ScannerInterface, cursors []ast.Position) scanner.ScannerInterface {
	return &autoCompleteScanner{scanner: scanner, cursors: append([]ast.Position{}, cursors...)}
}

func (acs *autoCompleteScanner) Scan() (tok scanner.Token) {
	if acs.nextToken != nil {
		tok = *acs.nextToken
		acs.nextToken = nil
		return
	}

	tok = acs.scanner.Scan()

	for i, pos := range acs.cursors {
		if tok.StartLine <= pos.Line &&
			tok.EndLine >= pos.Line &&
			tok.StartColumn <= pos.Column &&
			((pos.Line == tok.EndLine && tok.EndColumn >= pos.Column) || pos.Line < tok.EndLine) {
			if tok.Type == scanner.TokenTypeIdent {
				offset := pos.Column - tok.StartColumn
				tok.Text = tok.Text[0:offset] + "#" + tok.Text[offset:]
			} else if tok.Type != scanner.TokenTypeWhitespace {
				// Likely a member expression so lets add it after the next token
				acs.nextToken = &scanner.Token{
					Text:        "#",
					Value:       nil,
					Type:        scanner.TokenTypeIdent,
					StartColumn: pos.Column,
					EndColumn:   pos.Column + 1,
					StartLine:   pos.Line,
					EndLine:     pos.Line,
				}
			} else {
				acs.nextToken = &tok
				tok = scanner.Token{
					Text:        "#",
					Value:       nil,
					Type:        scanner.TokenTypeIdent,
					StartColumn: pos.Column,
					EndColumn:   pos.Column + 1,
					StartLine:   pos.Line,
					EndLine:     pos.Line,
				}
			}

			acs.cursors = append(acs.cursors[:i], acs.cursors[i+1:]...)

			return
		}
	}

	return
}
func (acs *autoCompleteScanner) SetErrorCallback(cb func(msg string)) {
	acs.scanner.SetErrorCallback(cb)
}

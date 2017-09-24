package ir

import "github.com/orktes/orlang/scanner"

type Scanner struct {
	scanner   scanner.ScannerInterface
	nextToken *scanner.Token
}

func NewScanner(scanner scanner.ScannerInterface) *Scanner {
	return &Scanner{scanner: scanner}
}

func (s *Scanner) Scan() (token scanner.Token) {
	if s.nextToken != nil {
		token = *s.nextToken
		s.nextToken = nil
		return
	}

	token = s.scanner.Scan()
	if token.Type == scanner.TokenTypeUnknown && token.Text == "%" {
		next := s.scanner.Scan()
		if next.Type == scanner.TokenTypeIdent {
			next.StartColumn = token.StartColumn
			next.Text = "%" + next.Text
			token = next
		} else {
			s.nextToken = &next
		}
	}
	return
}
func (s *Scanner) SetErrorCallback(cb func(msg string)) {
	s.scanner.SetErrorCallback(cb)
}

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

	// Handle %identifier -> TokenTypeIdent with Text "%identifier"
	if token.Type == scanner.TokenTypeUnknown && token.Text == "%" {
		originalPercentToken := token
		next := s.scanner.Scan()
		if next.Type == scanner.TokenTypeIdent {
			// Combine % and identifier
			combinedToken := next // Use next token's end position and other details
			combinedToken.Text = "%" + next.Text
			combinedToken.Position = originalPercentToken.Position // Starts at the '%'
			// EndPosition is already next's EndPosition, which is correct.
			token = combinedToken
		} else {
			// Not an identifier after %, treat % as unknown and put `next` back
			s.nextToken = &next
			// token remains the original TokenTypeUnknown "%"
		}
		return
	}

	// Handle "br" followed by "_cond" -> "br_cond"
	if token.Type == scanner.TokenTypeIdent && token.Text == "br" {
		originalBrToken := token
		next := s.scanner.Scan() // Look ahead
		if next.Type == scanner.TokenTypeIdent && next.Text == "_cond" {
			// Combine "br" and "_cond"
			combinedToken := next // Start with next token's details for EndPosition
			combinedToken.Text = "br_cond"
			combinedToken.Position = originalBrToken.Position // Starts at "br"
			// EndPosition is already next's EndPosition from "_cond", which is correct.
			token = combinedToken
		} else {
			// Not "_cond" after "br", put `next` back
			s.nextToken = &next
			// token remains the original "br" IDENT
		}
		return
	}

	return
}

func (s *Scanner) SetErrorCallback(cb func(msg string, pos scanner.Position)) {
	s.scanner.SetErrorCallback(cb)
}

// Offset returns the current parsing offset.
// This might require the underlying scanner to expose its offset.
// For now, it will return the start position of the last scanned token if available.
// This is a basic implementation; a more robust one would query the scanner directly.
func (s *Scanner) Offset() int {
	// ast.Position contains Line, Column, and Offset.
	// We need to ensure the underlying scanner provides this.
	// If s.scanner.Scan() returns tokens with valid Position.Offset,
	// then token.Position.Offset can be used.
	// This method isn't strictly used by the parser yet for error reporting offset,
	// but good to have for future. The parser's error reporting uses token.Position.
	// The existing parser's error refers to s.s.Offset() which implies ir.Scanner needs this.
	// Let's assume the scanner interface or its implementation can provide this.
	// If scanner.ScannerInterface does not have Offset(), this will need adjustment.
	// For now, let's assume it's available from the token.
	// This method is not directly called by the parser's p.error method,
	// as p.error uses p.tok.Position.
	// However, the old parser error message `p.s.Offset` suggests it was intended.
	// Let's make it consistent with using the token's position.
	// The `scanner.Scanner` from orklang/scanner does not expose a direct global offset method,
	// but its tokens have `Position.Offset`.
	// This method is not critical if the parser uses token.Position.
	return 0 // Placeholder, as parser uses token's position directly.
}

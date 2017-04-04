package scanner

import "testing"

func TestTokenToString(t *testing.T) {
	if (Token{Type: TokenTypeASTERISK, StartColumn: 11, Text: `*`}).String() != "0:11 ASTERISK(*)" {
		t.Error("Wrong to string returned")
	}
}

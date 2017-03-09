package scanner

import "testing"

func TestTokenToString(t *testing.T) {
	if (Token{Type: TokenTypeASTERIX, Column: 11, Text: `*`}).String() != "0:11 ASTERIX(*)" {
		t.Error("Wrong to string returned")
	}
}

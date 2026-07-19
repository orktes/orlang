package parser

import (
	"fmt"
	"strings"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

type PosError struct {
	ast.Position
	Message string
}

func (p PosError) Error() string {
	return fmt.Sprintf("%d:%d: %s", p.Position.Line+1, p.Position.Column+1, p.Message)
}

func unexpected(got string, expected string) string {
	return fmt.Sprintf("Expected %s got %s", expected, got)
}

// formatTokenTypes renders a list of expected token types as a readable
// alternation, e.g. "RPAREN or COMMA".
func formatTokenTypes(expected []scanner.TokenType) string {
	names := make([]string, len(expected))
	for i, t := range expected {
		names[i] = t.String()
	}
	if len(names) == 1 {
		return names[0]
	}
	return strings.Join(names[:len(names)-1], ", ") + " or " + names[len(names)-1]
}

func unexpectedToken(got scanner.Token, expected ...scanner.TokenType) string {
	if len(expected) == 0 {
		return fmt.Sprintf("Unexpected token %s", got.StringValue())
	}
	if got.Type == scanner.TokenTypeIdent {
		return fmt.Sprintf("Expected %s got %s", formatTokenTypes(expected), got.Text)
	}
	return fmt.Sprintf("Expected %s got %s", formatTokenTypes(expected), got.Type.String())
}

func reservedKeywordError(token scanner.Token) string {
	return fmt.Sprintf("%s is a reserved keyword", token.Text)
}

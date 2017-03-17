package parser

import (
	"fmt"

	"github.com/orktes/orlang/parser/ast"
	"github.com/orktes/orlang/parser/scanner"
)

type PosError struct {
	ast.Position
	Message string
}

func (p PosError) Error() string {
	return fmt.Sprintf("%d:%d: %s", p.Position.Line, p.Position.Column, p.Message)
}

func unexpected(got string, expected string) string {
	return fmt.Sprintf("Expected %s got %s", expected, got)
}

func unexpectedToken(got scanner.Token, expected ...scanner.TokenType) string {
	if len(expected) == 0 {
		return fmt.Sprintf("Unexpected token %s", got.Type.String())
	}
	return fmt.Sprintf("Expected %s got %s", expected, got.Type.String())
}

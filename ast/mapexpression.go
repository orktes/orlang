package ast

import (
	"fmt"
	"strings"

	"github.com/orktes/orlang/scanner"
)

type MapExpression struct {
	Type       *MapType
	LeftBrace  scanner.Token
	RightBrace scanner.Token
	Entries    []*MapEntry
}

type MapEntry struct {
	Key   Expression
	Colon scanner.Token
	Value Expression
}

func (MapExpression) exprNode() {}

func (me *MapExpression) StartPos() Position {
	return me.Type.StartPos()
}

func (me *MapExpression) EndPos() Position {
	return EndPositionFromToken(me.RightBrace)
}

func (me *MapExpression) String() string {
	entries := make([]string, len(me.Entries))
	for i, entry := range me.Entries {
		entries[i] = fmt.Sprintf("%s: %s", entry.Key, entry.Value)
	}
	return fmt.Sprintf("%s{%s}", me.Type, strings.Join(entries, ", "))
}

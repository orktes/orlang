package ast

import (
	"fmt"
	"strings"
)

type FunctionCall struct {
	Callee    Expression
	End       Position
	Arguments []*CallArgument
}

func (fc *FunctionCall) StartPos() Position {
	return fc.Callee.StartPos()
}

func (fc *FunctionCall) EndPos() Position {
	return fc.End
}

func (fc *FunctionCall) String() string {
	names := []string{}

	for _, arg := range fc.Arguments {
		if arg.Name != nil {
			names = append(names, fmt.Sprintf("%s: %s", arg.Name.Text, arg.Expression))
		} else {
			names = append(names, fmt.Sprintf("%s", arg.Expression))
		}
	}
	return fmt.Sprintf("%s(%s)", fc.Callee, strings.Join(names, ", "))
}

func (_ FunctionCall) exprNode() {
}

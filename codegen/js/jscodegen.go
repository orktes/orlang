package js

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/orktes/orlang/analyser"
	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/types"
)

type JSCodeGen struct {
	analyserInfo *analyser.Info
	buffer       bytes.Buffer
	currentFile  *ast.File
}

func New(info *analyser.Info) *JSCodeGen {
	return &JSCodeGen{analyserInfo: info}
}

func (jscg *JSCodeGen) getTempVar() string {
	return "_temp"
}

func (jscg *JSCodeGen) Visit(node ast.Node) ast.Visitor {
	nodeInfo := jscg.analyserInfo.FileInfo[jscg.currentFile].NodeInfo[node]
	switch n := node.(type) {
	case *ast.File, *ast.Macro, *ast.CallArgument:
	case *ast.TupleDeclaration:
		tempVar := jscg.getTempVar()
		jscg.buffer.WriteString(fmt.Sprintf(
			`var %s`,
			tempVar,
		))

		if n.DefaultValue != nil {
			jscg.buffer.WriteString("=")
		}

		ast.Walk(jscg, n.DefaultValue)

		jscg.buffer.WriteString(";")

		var ptrnPrint func(pattern *ast.TuplePattern, prefix string)
		ptrnPrint = func(pattern *ast.TuplePattern, prefix string) {
			for i, ptrrn := range pattern.Patterns {
				switch pt := ptrrn.(type) {
				case *ast.TuplePattern:
					ptrnPrint(pt, fmt.Sprintf("%s[%d];", prefix, i))
				case *ast.Identifier:
					jscg.buffer.WriteString(
						fmt.Sprintf(
							"var %s = %s[%d];",
							pt.Text,
							prefix,
							i,
						))
				}

			}
		}

		ptrnPrint(n.Pattern, tempVar)

		return nil
	case *ast.VariableDeclaration:
		jscg.buffer.WriteString(fmt.Sprintf(
			`var %s`,
			n.Name.Text,
		))
		if n.DefaultValue != nil {
			jscg.buffer.WriteString("=")
		}

		ast.Walk(jscg, n.DefaultValue)
		return nil
	case *ast.TupleExpression:
		jscg.buffer.WriteString(`[`)

		// TODO set named args into the right order
		for i, expr := range n.Expressions {
			ast.Walk(jscg, expr)
			if i < len(n.Expressions)-1 {
				jscg.buffer.WriteString(`,`)
			}
		}

		jscg.buffer.WriteString(`]`)
		return nil
	case *ast.UnaryExpression:
		jscg.buffer.WriteString(n.Operator.Text)
	case *ast.BinaryExpression:
		ast.Walk(jscg, n.Left)
		jscg.buffer.WriteString(n.Operator.Text)
		ast.Walk(jscg, n.Right)
		return nil
	case *ast.Identifier:
		if n == nil {
			break
		}
		jscg.buffer.WriteString(n.Text)
	case *ast.ValueExpression:
		jscg.buffer.WriteString(fmt.Sprintf(
			`%s`,
			n.Text,
		))
	case *ast.ReturnStatement:
		jscg.buffer.WriteString(`return `)
	case *ast.FunctionCall:
		if nodeInfo.TypeCast {
			if nodeInfo.Type == types.Int32Type || nodeInfo.Type == types.Int64Type {
				jscg.buffer.WriteString(`Math.floor`)
			} else {
				ast.Walk(jscg, n.Arguments[0])
				return nil
			}
		} else {
			ast.Walk(jscg, n.Callee)
		}
		jscg.buffer.WriteString(`(`)

		// TODO set named args into the right order
		for i, expr := range n.Arguments {
			ast.Walk(jscg, expr)
			if i < len(n.Arguments)-1 {
				jscg.buffer.WriteString(`,`)
			}
		}

		jscg.buffer.WriteString(`)`)

		return nil
	case *ast.Block:
		jscg.buffer.WriteString(`{`)
	case *ast.FunctionDeclaration:
		var name string
		var args []string
		if n.Signature.Identifier != nil {
			name = n.Signature.Identifier.Text
		}

		for _, arg := range n.Signature.Arguments {
			args = append(args, arg.Name.Text)
		}

		jscg.buffer.WriteString(fmt.Sprintf(
			`function %s (%s)`,
			name,
			strings.Join(args, ","),
		))

		ast.Walk(jscg, n.Block)

		return nil
	default:
		panic(fmt.Sprintf("TODO: %T", n))
	}
	return jscg
}

func (jscg *JSCodeGen) Leave(node ast.Node) {
	switch node.(type) {
	case *ast.TupleDeclaration:
		// DO nothing
	case *ast.File:
		jscg.buffer.WriteString("main && main();")
	case *ast.Block:
		jscg.buffer.WriteString(`}`)
	case ast.Statement, *ast.FunctionDeclaration:
		jscg.buffer.WriteString(`;`)
	}
}

func (jscg *JSCodeGen) Generate(file *ast.File) []byte {
	jscg.currentFile = file
	ast.Walk(jscg, file)
	return jscg.buffer.Bytes()
}

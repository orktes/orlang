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
	identNumbers map[*ast.Identifier]int
}

func New(info *analyser.Info) *JSCodeGen {
	return &JSCodeGen{analyserInfo: info, identNumbers: map[*ast.Identifier]int{}}
}

func (jscg *JSCodeGen) getTempVar() string {
	// TODO check if temp var is defined in scope
	return "_temp"
}

func (jscg *JSCodeGen) getIdentifier(ident *ast.Identifier) string {
	nodeInfo := jscg.analyserInfo.FileInfo[jscg.currentFile].NodeInfo[ident]
	scopeItemDetals := nodeInfo.Scope.GetDetails(ident.Text, true)

	identNumber := 0
	if number, ok := jscg.identNumbers[scopeItemDetals.DefineIdentifier]; ok {
		identNumber = number
	} else {
		identNumber = len(jscg.identNumbers)
		jscg.identNumbers[scopeItemDetals.DefineIdentifier] = identNumber
	}

	return fmt.Sprintf("$%d", identNumber)
}

func (jscg *JSCodeGen) Visit(node ast.Node) ast.Visitor {
	nodeInfo := jscg.analyserInfo.FileInfo[jscg.currentFile].NodeInfo[node]
	switch n := node.(type) {
	case *ast.Macro, *ast.CallArgument:
	case *ast.TupleDeclaration:

		var varName string

		if n.DefaultValue != nil {
			if ident, ok := n.DefaultValue.(*ast.Identifier); ok {
				varName = jscg.getIdentifier(ident)
			}
		}

		if varName == "" {
			varName = jscg.getTempVar()
			jscg.buffer.WriteString(fmt.Sprintf(
				`var %s`,
				varName,
			))

			if n.DefaultValue != nil {
				jscg.buffer.WriteString("=")
			}

			ast.Walk(jscg, n.DefaultValue)

			jscg.buffer.WriteString(";")
		}

		var ptrnPrint func(pattern *ast.TuplePattern, prefix string)
		ptrnPrint = func(pattern *ast.TuplePattern, prefix string) {
			for i, ptrrn := range pattern.Patterns {
				switch pt := ptrrn.(type) {
				case *ast.TuplePattern:
					ptrnPrint(pt, fmt.Sprintf("%s[%d]", prefix, i))
				case *ast.Identifier:
					jscg.buffer.WriteString(
						fmt.Sprintf(
							"var %s = %s[%d]",
							jscg.getIdentifier(pt),
							prefix,
							i,
						))
					if i != len(pattern.Patterns)-1 {
						jscg.buffer.WriteString(";")
					}
				}

			}
		}

		ptrnPrint(n.Pattern, varName)

		return nil
	case *ast.VariableDeclaration:
		jscg.buffer.WriteString(fmt.Sprintf(
			`var %s`,
			jscg.getIdentifier(n.Name),
		))
		if n.DefaultValue != nil {
			jscg.buffer.WriteString("=")
		}

		ast.Walk(jscg, n.DefaultValue)
		return nil
	case *ast.IfStatement:
		jscg.buffer.WriteString(" if (")
		ast.Walk(jscg, n.Condition)
		jscg.buffer.WriteString(")")

		ast.Walk(jscg, n.Block)

		if n.Else != nil {
			jscg.buffer.WriteString(" else ")
			ast.Walk(jscg, n.Else)
		}

		return nil
	case *ast.TupleExpression:
		jscg.buffer.WriteString(`[`)

		for i, expr := range n.Expressions {
			ast.Walk(jscg, expr)
			if i < len(n.Expressions)-1 {
				jscg.buffer.WriteString(`,`)
			}
		}

		jscg.buffer.WriteString(`]`)
		return nil
	case *ast.UnaryExpression:
		if n.Postfix {
			ast.Walk(jscg, n.Expression)

		}

		jscg.buffer.WriteString(n.Operator.Text)

		if n.Postfix {
			return nil
		}
	case *ast.ComparisonExpression:
		ast.Walk(jscg, n.Left)
		jscg.buffer.WriteString(n.Operator.Text)
		ast.Walk(jscg, n.Right)
		return nil
	case *ast.BinaryExpression:
		ast.Walk(jscg, n.Left)
		jscg.buffer.WriteString(n.Operator.Text)
		ast.Walk(jscg, n.Right)
		return nil
	case *ast.Identifier:
		if n == nil {
			break
		}
		if sigType, ok := nodeInfo.Type.(*types.SignatureType); ok {
			if sigType.Extern {
				jscg.buffer.WriteString(n.Text)
				break
			}
		}

		jscg.buffer.WriteString(jscg.getIdentifier(n))
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
		var argNames []string

		calleeNodeInfo := jscg.analyserInfo.FileInfo[jscg.currentFile].NodeInfo[n.Callee]
		if calleeNodeInfo != nil && calleeNodeInfo.Type != nil {
			argNames = calleeNodeInfo.Type.(*types.SignatureType).ArgumentNames
		}

		namedArgs := len(n.Arguments) > 0 && n.Arguments[0].Name != nil
		if len(argNames) == 0 || !namedArgs {
			for i, expr := range n.Arguments {
				ast.Walk(jscg, expr)
				if i < len(n.Arguments)-1 {
					jscg.buffer.WriteString(`,`)
				}
			}
		} else {
			for i, argName := range argNames {
				found := false
				for _, expr := range n.Arguments {
					if expr.Name.Text == argName {
						ast.Walk(jscg, expr.Expression)
						found = true
						break
					}
				}

				if !found {
					jscg.buffer.WriteString("undefined")
				}

				if i < len(argNames)-1 {
					jscg.buffer.WriteString(`,`)
				}
			}
		}

		jscg.buffer.WriteString(`)`)

		return nil
	case *ast.Block:
		jscg.buffer.WriteString(`{`)

		for _, node := range n.Body {
			ast.Walk(jscg, node)
			jscg.buffer.WriteString(";")
		}

		return nil
	case *ast.File:
		jscg.buffer.WriteString("(function () {")
		for _, node := range n.Body {
			ast.Walk(jscg, node)
			jscg.buffer.WriteString(";")
		}
		return nil
	case *ast.ForLoop:
		jscg.buffer.WriteString(" for (")

		if n.Init != nil {
			ast.Walk(jscg, n.Init)
		}
		jscg.buffer.WriteString(";")

		if n.Condition != nil {
			ast.Walk(jscg, n.Condition)
		}
		jscg.buffer.WriteString(";")

		if n.After != nil {
			ast.Walk(jscg, n.After)
		}
		jscg.buffer.WriteString(")")

		ast.Walk(jscg, n.Block)
		return nil
	case *ast.FunctionDeclaration:
		var name string
		var args []string
		if n.Signature.Identifier != nil {
			name = jscg.getIdentifier(n.Signature.Identifier)
		}

		for _, arg := range n.Signature.Arguments {
			args = append(args, jscg.getIdentifier(arg.Name))
		}

		jscg.buffer.WriteString(fmt.Sprintf(
			`function %s (%s)`,
			name,
			strings.Join(args, ","),
		))

		ast.Walk(jscg, n.Block)

		return nil
	case *ast.ParenExpression:
		jscg.buffer.WriteString("(")
	default:
		panic(fmt.Sprintf("TODO: %T", n))
	}
	return jscg
}

func (jscg *JSCodeGen) Leave(node ast.Node) {
	switch n := node.(type) {
	case *ast.TupleDeclaration:
		// DO nothing
	case *ast.File:
		for _, n := range n.Body {
			if funDecl, ok := n.(*ast.FunctionDeclaration); ok {
				if funDecl.Signature != nil && funDecl.Signature.Identifier != nil && funDecl.Signature.Identifier.Text == "main" {
					name := jscg.getIdentifier(funDecl.Signature.Identifier)
					jscg.buffer.WriteString(fmt.Sprintf("%s && %s();", name, name))
					break
				}
			}
		}

		jscg.buffer.WriteString("})();")
	case *ast.Block:
		jscg.buffer.WriteString(`}`)
	case *ast.ParenExpression:
		jscg.buffer.WriteString(")")
	}
}

func (jscg *JSCodeGen) Generate(file *ast.File) []byte {
	jscg.currentFile = file
	ast.Walk(jscg, file)
	return jscg.buffer.Bytes()
}
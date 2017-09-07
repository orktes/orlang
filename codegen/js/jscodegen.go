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
	identNumbers map[ast.Node]int
	types        map[string]ast.Node
}

func New(info *analyser.Info) *JSCodeGen {
	return &JSCodeGen{analyserInfo: info, identNumbers: map[ast.Node]int{}, types: map[string]ast.Node{}}
}

func (jscg *JSCodeGen) getIdentifierForNode(node ast.Node, name string) string {
	identNumber := 0
	if number, ok := jscg.identNumbers[node]; ok {
		identNumber = number
	} else {
		identNumber = len(jscg.identNumbers)
		jscg.identNumbers[node] = identNumber
	}

	return fmt.Sprintf("$%d_%s", identNumber, name)
}

func (jscg *JSCodeGen) getIdentifier(ident *ast.Identifier) string {
	nodeInfo := jscg.analyserInfo.FileInfo[jscg.currentFile].NodeInfo[ident]
	scopeItemDetals := nodeInfo.Scope.GetDetails(ident.Text, true)
	return jscg.getIdentifierForNode(scopeItemDetals.DefineIdentifier, ident.Text)
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
			varName = jscg.getIdentifierForNode(n, "tuple_temp")
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
	case *ast.Assigment:
		ast.Walk(jscg, n.Left)
		jscg.buffer.WriteString(" = ")
		ast.Walk(jscg, n.Right)
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
		if nodeInfo.OverloadedOperation == nil {
			ast.Walk(jscg, n.Left)
			jscg.buffer.WriteString(n.Operator.Text)
			ast.Walk(jscg, n.Right)
		} else {
			// Operator has been overloaded
			name := jscg.getIdentifierForNode(nodeInfo.OverloadedOperation, n.Operator.Type.String())
			jscg.buffer.WriteString(fmt.Sprintf("%s(", name))
			ast.Walk(jscg, n.Left)
			jscg.buffer.WriteString(", ")
			ast.Walk(jscg, n.Right)
			jscg.buffer.WriteString(")")
		}
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

		jscg.buffer.WriteString(`}`)
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
		if _, isStruct := nodeInfo.Parent.(*ast.Struct); !isStruct {
			if n.Signature.Identifier != nil {
				name = jscg.getIdentifier(n.Signature.Identifier)
			} else if n.Signature.Operator != nil {
				// Operator overload
				name = jscg.getIdentifierForNode(n, n.Signature.Operator.Type.String())
			}
		}

		for _, arg := range n.Signature.Arguments {
			args = append(args, jscg.getIdentifier(arg.Name))
		}

		jscg.buffer.WriteString(fmt.Sprintf(
			`function %s (%s)`,
			name,
			strings.Join(args, ","),
		))

		jscg.buffer.WriteString("{")

		for _, arg := range n.Signature.Arguments {
			if arg.DefaultValue != nil {
				name := jscg.getIdentifier(arg.Name)
				jscg.buffer.WriteString(fmt.Sprintf(
					`if (%s === undefined) {%s =`,
					name,
					name,
				))
				ast.Walk(jscg, arg.DefaultValue)
				jscg.buffer.WriteString("}")
			}
		}

		for _, node := range n.Block.Body {
			ast.Walk(jscg, node)
			jscg.buffer.WriteString(";")
		}

		jscg.buffer.WriteString("}")
		return nil
	case *ast.ParenExpression:
		jscg.buffer.WriteString("(")
	case *ast.Struct:
		if n.Name == nil {
			break
		}

		name := jscg.getIdentifierForNode(n, n.Name.Text)
		jscg.types[n.Name.Text] = n

		args := []string{}
		for _, v := range n.Variables {
			args = append(args, v.Name.Text)
		}

		jscg.buffer.WriteString(fmt.Sprintf("function %s (%s) {", name, strings.Join(args, ", ")))
		for _, v := range n.Variables {
			name := v.Name.Text
			if v.DefaultValue != nil {
				jscg.buffer.WriteString(fmt.Sprintf(
					`this.%s = %s !== undefined ? %s : `,
					name,
					name,
					name,
				))
				ast.Walk(jscg, v.DefaultValue)
			} else {
				jscg.buffer.WriteString(fmt.Sprintf(
					`this.%s = %s;`,
					name,
					name,
				))
			}

			jscg.buffer.WriteString(";")
		}

		jscg.buffer.WriteString("};")

		for i, funDecl := range n.Functions {
			var funcName string
			if funDecl.Signature.Identifier != nil {
				funcName = funDecl.Signature.Identifier.Text
			} else if funDecl.Signature.Operator != nil {
				// Operator overload
				funcName = jscg.getIdentifierForNode(funDecl, funDecl.Signature.Operator.Type.String())
				jscg.buffer.WriteString(fmt.Sprintf("var %s =", funcName))
			}

			jscg.buffer.WriteString(fmt.Sprintf(
				"%s.prototype.%s = ",
				name,
				funcName,
			))

			ast.Walk(jscg, funDecl)
			if i < len(n.Functions)-1 {
				jscg.buffer.WriteString(";")
			}
		}

		return nil
	case *ast.StructExpression:
		if typeNode := jscg.types[n.Identifier.Text]; typeNode != nil {
			if structTypeNode, ok := typeNode.(*ast.Struct); ok {
				name := jscg.getIdentifierForNode(structTypeNode, structTypeNode.Name.Text)
				jscg.buffer.WriteString(fmt.Sprintf("new %s()", name))
				// TODO struct args
			}
		}
		return nil
	case *ast.MemberExpression:
		ast.Walk(jscg, n.Target)
		jscg.buffer.WriteString(".")
		jscg.buffer.WriteString(n.Property.Text)

		return nil
	default:
		panic(fmt.Sprintf("TODO: %T", n))
	}
	return jscg
}

func (jscg *JSCodeGen) Leave(node ast.Node) {
	switch n := node.(type) {
	case *ast.File:
		for _, n := range n.Body {
			if funDecl, ok := n.(*ast.FunctionDeclaration); ok {
				if funDecl.Signature != nil && funDecl.Signature.Identifier != nil && funDecl.Signature.Identifier.Text == "main" {
					name := jscg.getIdentifier(funDecl.Signature.Identifier)
					jscg.buffer.WriteString(fmt.Sprintf("%s();", name))
					break
				}
			}
		}

		jscg.buffer.WriteString("})();")
	case *ast.ParenExpression:
		jscg.buffer.WriteString(")")
	}
}

func (jscg *JSCodeGen) Generate(file *ast.File) []byte {
	jscg.currentFile = file
	ast.Walk(jscg, file)
	return jscg.buffer.Bytes()
}

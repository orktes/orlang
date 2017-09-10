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

func (jscg *JSCodeGen) getParent(node ast.Node) ast.Node {

	return jscg.getNodeInfo(node).Parent.Node
}

func (jscg *JSCodeGen) getNodeInfo(node ast.Node) *analyser.NodeInfo {
	return jscg.analyserInfo.FileInfo[jscg.currentFile].NodeInfo[node]
}

func (jscg *JSCodeGen) writeWithNodePosition(node ast.Node, str string) {
	jscg.writeWithPosition(node.StartPos(), node.EndPos(), str)
}

func (jscg *JSCodeGen) writeWithPosition(start ast.Position, end ast.Position, str string) {
	jscg.write(str)
}

func (jscg *JSCodeGen) write(str string) {
	jscg.buffer.WriteString(str)
}

func (jscg *JSCodeGen) Visit(node ast.Node) ast.Visitor {
	nodeInfo := jscg.analyserInfo.FileInfo[jscg.currentFile].NodeInfo[node]
	switch n := node.(type) {
	case *ast.Macro:
	case *ast.CallArgument:
		ast.Walk(jscg, n.Expression)
		return nil
	case *ast.TupleDeclaration:

		var varName string

		if n.DefaultValue != nil {
			if ident, ok := n.DefaultValue.(*ast.Identifier); ok {
				varName = jscg.getIdentifier(ident)
			}
		}

		if varName == "" {
			varName = jscg.getIdentifierForNode(n, "tuple_temp")
			jscg.writeWithNodePosition(n, fmt.Sprintf(
				`var %s`,
				varName,
			))

			if n.DefaultValue != nil {
				jscg.write("=")
			}

			ast.Walk(jscg, n.DefaultValue)

			jscg.write(";")
		}

		var ptrnPrint func(pattern *ast.TuplePattern, prefix string)
		ptrnPrint = func(pattern *ast.TuplePattern, prefix string) {
			for i, ptrrn := range pattern.Patterns {
				switch pt := ptrrn.(type) {
				case *ast.TuplePattern:
					ptrnPrint(pt, fmt.Sprintf("%s[%d]", prefix, i))
				case *ast.Identifier:
					jscg.writeWithNodePosition(pt,
						fmt.Sprintf(
							"var %s = %s[%d]",
							jscg.getIdentifier(pt),
							prefix,
							i,
						))
					if i != len(pattern.Patterns)-1 {
						jscg.write(";")
					}
				}

			}
		}

		ptrnPrint(n.Pattern, varName)

		return nil
	case *ast.VariableDeclaration:
		jscg.writeWithNodePosition(n.Name, fmt.Sprintf(
			`var %s`,
			jscg.getIdentifier(n.Name),
		))
		if n.DefaultValue != nil {
			jscg.write("=")
		}

		ast.Walk(jscg, n.DefaultValue)
		return nil
	case *ast.Assigment:
		ast.Walk(jscg, n.Left)
		jscg.write(" = ")
		ast.Walk(jscg, n.Right)
		return nil
	case *ast.IfStatement:
		jscg.writeWithPosition(n.StartPos(), n.StartPos(), " if (")
		ast.Walk(jscg, n.Condition)
		jscg.write(")")

		ast.Walk(jscg, n.Block)

		if n.Else != nil {
			jscg.writeWithNodePosition(n.Else, " else ")
			ast.Walk(jscg, n.Else)
		}

		return nil
	case *ast.TupleExpression:
		jscg.writeWithPosition(n.StartPos(), n.EndPos(), `[`)

		for i, expr := range n.Expressions {
			ast.Walk(jscg, expr)
			if i < len(n.Expressions)-1 {
				jscg.write(`,`)
			}
		}

		jscg.writeWithPosition(n.EndPos(), n.EndPos(), `]`)
		return nil
	case *ast.UnaryExpression:
		if n.Postfix {
			ast.Walk(jscg, n.Expression)

		}

		jscg.writeWithPosition(ast.StartPositionFromToken(n.Operator), ast.EndPositionFromToken(n.Operator), n.Operator.Text)

		if n.Postfix {
			return nil
		}
	case *ast.ComparisonExpression:
		ast.Walk(jscg, n.Left)
		jscg.writeWithPosition(ast.StartPositionFromToken(n.Operator), ast.EndPositionFromToken(n.Operator), n.Operator.Text)
		ast.Walk(jscg, n.Right)
		return nil
	case *ast.BinaryExpression:
		if nodeInfo.OverloadedOperation == nil {
			ast.Walk(jscg, n.Left)
			jscg.writeWithPosition(ast.StartPositionFromToken(n.Operator), ast.EndPositionFromToken(n.Operator), n.Operator.Text)
			ast.Walk(jscg, n.Right)
		} else {
			// Operator has been overloaded
			name := jscg.getIdentifierForNode(nodeInfo.OverloadedOperation, n.Operator.Type.String())

			// If Overload operation parent is a struct refer to it trough that
			parent := jscg.getParent(nodeInfo.OverloadedOperation)
			if parentStruct, parentStructOk := parent.(*ast.Struct); parentStructOk {
				name = fmt.Sprintf(
					"%s.prototype.%s",
					jscg.getIdentifierForNode(parentStruct, parentStruct.Name.Text),
					name,
				)
			}

			jscg.writeWithPosition(ast.StartPositionFromToken(n.Operator), ast.EndPositionFromToken(n.Operator), fmt.Sprintf("%s(", name))
			ast.Walk(jscg, n.Left)
			jscg.write(", ")
			ast.Walk(jscg, n.Right)
			jscg.writeWithPosition(n.EndPos(), n.EndPos(), ")")
		}
		return nil
	case *ast.Identifier:
		if n == nil {
			break
		}

		if sigType, ok := nodeInfo.Type.(*types.SignatureType); ok {
			if sigType.Extern {
				jscg.writeWithNodePosition(n, n.Text)
				break
			}
		}

		if n.Text == "this" {
			jscg.writeWithNodePosition(n, "this")
			break
		}

		jscg.writeWithNodePosition(n, jscg.getIdentifier(n))
	case *ast.ValueExpression:
		jscg.writeWithNodePosition(n, fmt.Sprintf(
			`%s`,
			n.Text,
		))
	case *ast.ReturnStatement:
		jscg.writeWithPosition(n.Start, n.ReturnEnd, `return `)
	case *ast.FunctionCall:
		if nodeInfo.TypeCast {
			if nodeInfo.Type == types.Int32Type || nodeInfo.Type == types.Int64Type {
				jscg.writeWithNodePosition(n, `Math.floor`)
			} else {
				ast.Walk(jscg, n.Arguments[0])
				return nil
			}
		} else {
			ast.Walk(jscg, n.Callee)
		}
		jscg.write(`(`)

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
					jscg.write(`,`)
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
					jscg.write("undefined")
				}

				if i < len(argNames)-1 {
					jscg.write(`,`)
				}
			}
		}

		jscg.write(`)`)

		return nil
	case *ast.Block:
		jscg.writeWithPosition(n.Start, n.Start, `{`)

		for _, node := range n.Body {
			ast.Walk(jscg, node)
			jscg.write(";")
		}

		jscg.writeWithPosition(n.End, n.End, `}`)
		return nil
	case *ast.File:
		jscg.write("(function () {")
		for _, node := range n.Body {
			ast.Walk(jscg, node)
			jscg.write(";")
		}
		return nil
	case *ast.ForLoop:
		jscg.writeWithPosition(n.StartPos(), n.StartPos(), " for (")

		if n.Init != nil {
			ast.Walk(jscg, n.Init)
		}

		jscg.write(";")

		if n.Condition != nil {
			ast.Walk(jscg, n.Condition)
		}
		jscg.write(";")

		if n.After != nil {
			ast.Walk(jscg, n.After)
		}
		jscg.write(")")

		ast.Walk(jscg, n.Block)
		return nil
	case *ast.FunctionDeclaration:
		var name string
		var args []string
		var start ast.Position
		var end ast.Position
		if _, isStruct := nodeInfo.Parent.Node.(*ast.Struct); !isStruct {
			if n.Signature.Identifier != nil {
				name = jscg.getIdentifier(n.Signature.Identifier)
				start = n.Signature.Identifier.StartPos()
				end = n.Signature.Identifier.EndPos()
			} else if n.Signature.Operator != nil {
				// Operator overload
				start = ast.StartPositionFromToken(*n.Signature.Operator)
				end = ast.EndPositionFromToken(*n.Signature.Operator)
				name = jscg.getIdentifierForNode(n, n.Signature.Operator.Type.String())
			}
		}

		for _, arg := range n.Signature.Arguments {
			args = append(args, jscg.getIdentifier(arg.Name))
		}

		jscg.writeWithPosition(start, end, fmt.Sprintf(
			`function %s (%s)`,
			name,
			strings.Join(args, ","),
		))

		jscg.write("{")

		for _, arg := range n.Signature.Arguments {
			if arg.DefaultValue != nil {
				name := jscg.getIdentifier(arg.Name)
				jscg.writeWithNodePosition(arg.DefaultValue, fmt.Sprintf(
					`if (%s === undefined) {%s =`,
					name,
					name,
				))
				ast.Walk(jscg, arg.DefaultValue)
				jscg.write("}")
			}
		}

		for _, node := range n.Block.Body {
			ast.Walk(jscg, node)
			jscg.write(";")
		}

		jscg.write("}")
		return nil
	case *ast.ParenExpression:
		jscg.writeWithPosition(n.StartPos(), n.StartPos(), "(")
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

		jscg.writeWithNodePosition(n.Name, fmt.Sprintf("function %s (%s) {", name, strings.Join(args, ", ")))
		for _, v := range n.Variables {
			name := v.Name.Text
			if v.DefaultValue != nil {
				jscg.writeWithNodePosition(v, fmt.Sprintf(
					`this.%s = %s !== undefined ? %s : `,
					name,
					name,
					name,
				))
				ast.Walk(jscg, v.DefaultValue)
			} else {
				jscg.writeWithNodePosition(v, fmt.Sprintf(
					`this.%s = %s;`,
					name,
					name,
				))
			}

			jscg.write(";")
		}

		jscg.write("};")

		for i, funDecl := range n.Functions {
			var start ast.Position
			var end ast.Position
			var funcName string
			if funDecl.Signature.Identifier != nil {
				funcName = funDecl.Signature.Identifier.Text
				start = funDecl.Signature.Identifier.StartPos()
				end = funDecl.Signature.Identifier.EndPos()
			} else if funDecl.Signature.Operator != nil {
				// Operator overload
				funcName = jscg.getIdentifierForNode(funDecl, funDecl.Signature.Operator.Type.String())
				start = ast.StartPositionFromToken(*funDecl.Signature.Operator)
				end = ast.EndPositionFromToken(*funDecl.Signature.Operator)
			}

			jscg.writeWithPosition(start, end, fmt.Sprintf(
				"%s.prototype.%s = ",
				name,
				funcName,
			))

			ast.Walk(jscg, funDecl)
			if i < len(n.Functions)-1 {
				jscg.write(";")
			}
		}

		return nil
	case *ast.Interface:
		if n.Name == nil {
			break
		}

		name := jscg.getIdentifierForNode(n, n.Name.Text)
		jscg.types[n.Name.Text] = n

		jscg.writeWithNodePosition(n.Name, fmt.Sprintf("function %s (val) {};", name))

		for _, signature := range n.Functions {
			var start ast.Position
			var end ast.Position
			var funcName string
			if signature.Identifier != nil {
				funcName = signature.Identifier.Text
				start = signature.Identifier.StartPos()
				end = signature.Identifier.EndPos()
			} else if signature.Operator != nil {
				// Here be dragons
				// TODO decide what to do here. Most likely analyzer should trough an error
			}

			jscg.writeWithPosition(start, end, fmt.Sprintf(
				"%s.prototype.%s = function () { return this.%s.apply(this, arguments); };",
				name,
				funcName,
				funcName,
			))
		}

		return nil
	case *ast.StructExpression:
		if typeNode := jscg.types[n.Identifier.Text]; typeNode != nil {
			if structTypeNode, ok := typeNode.(*ast.Struct); ok {
				name := jscg.getIdentifierForNode(structTypeNode, structTypeNode.Name.Text)
				jscg.writeWithNodePosition(n, fmt.Sprintf("new %s(", name))

				var argNames []string

				for _, varDef := range structTypeNode.Variables {
					argNames = append(argNames, varDef.Name.Text)
				}

				namedArgs := len(n.Arguments) > 0 && n.Arguments[0].Name != nil
				if len(argNames) == 0 || !namedArgs {
					for i, expr := range n.Arguments {
						ast.Walk(jscg, expr)
						if i < len(n.Arguments)-1 {
							jscg.write(`,`)
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
							jscg.write("undefined")
						}

						if i < len(argNames)-1 {
							jscg.write(`,`)
						}
					}
				}

				jscg.write(")")
			}
		}

		return nil
	case *ast.MemberExpression:
		// TODO clean this up
		targetType := jscg.getNodeInfo(n.Target).Type
		if structType, structTypeOk := targetType.(types.TypeWithMethods); structTypeOk {
			// Is property a method
			if ok, _ := structType.HasFunction(n.Property.Text); ok {
				// Is it a function call
				if _, parentIsFunctionCall := nodeInfo.Parent.Node.(*ast.FunctionCall); !parentIsFunctionCall {
					// Get targetet struct

					name := ""
					switch typ := structType.(type) {
					case *types.StructType:
						name = typ.Name
					case *types.InterfaceType:
						name = typ.Name
					}

					if typeNode := jscg.types[name]; typeNode != nil {
						name := jscg.getIdentifierForNode(typeNode, name)
						jscg.writeWithPosition(
							node.StartPos(),
							node.StartPos(),
							fmt.Sprintf(
								"%s.prototype.%s.bind(",
								name,
								n.Property.Text,
							),
						)

						ast.Walk(jscg, n.Target)
						jscg.write(")")

						return nil

					}
				}
			}
		}

		ast.Walk(jscg, n.Target)
		jscg.write(".")
		jscg.writeWithNodePosition(n.Property, n.Property.Text)

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
					jscg.write(fmt.Sprintf("%s();", name))
					break
				}
			}
		}

		jscg.write("})();")
	case *ast.ParenExpression:
		jscg.writeWithPosition(n.EndPos(), n.EndPos(), ")")
	}
}

func (jscg *JSCodeGen) Generate(file *ast.File) []byte {
	jscg.currentFile = file
	ast.Walk(jscg, file)
	return jscg.buffer.Bytes()
}

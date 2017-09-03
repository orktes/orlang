package analyser

import (
	"fmt"
	"math"
	"reflect"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
	"github.com/orktes/orlang/types"
)

type visitor struct {
	node    ast.Node
	scope   *Scope
	info    *FileInfo
	parent  *visitor
	errorCb func(node ast.Node, msg string, fatal bool)
}

func (v *visitor) subVisitor(node ast.Node, scope *Scope) *visitor {
	return &visitor{info: v.info, parent: v, node: node, scope: scope, errorCb: v.errorCb}
}

func (v *visitor) emitError(node ast.Node, err string, fatal bool) {
	if v.errorCb != nil {
		v.errorCb(node, err, fatal)
	}
}

func (v *visitor) scopeMustGet(identifier *ast.Identifier, cb func(ScopeItem)) {
	if node := v.scope.Get(identifier.Text, true); node != nil {
		cb(node)
	}
}

func (v *visitor) getTypesForNodeList(nodes ...ast.Node) []types.Type {
	types := make([]types.Type, len(nodes))

	for i, node := range nodes {
		types[i] = v.getTypeForNode(node)
	}

	return types
}

func (v *visitor) getTypeForNode(node ast.Node) types.Type {
	switch n := node.(type) {
	case *ast.ArrayType:
		arrLength := int64(-1)
		if valExpr, ok := n.Length.(*ast.ValueExpression); ok {
			if valExpr.Token.Type != scanner.TokenTypeNumber {
				v.emitError(valExpr, "array length must be an integer", true)
				break
			}

			arrLength = valExpr.Token.Value.(int64)
		}

		return &types.ArrayType{
			Type:   v.getTypeForNode(n.Type),
			Length: arrLength,
		}
	case *ast.ArrayExpression:
		return &types.ArrayType{
			Type:   v.getTypeForNode(n.Type.Type),
			Length: int64(len(n.Expressions)),
		}
	case *ast.VariableDeclaration:
		if n.Type != nil {
			return v.getTypeForNode(n.Type)
		}
		return v.getTypeForNode(n.DefaultValue)
	case *ast.TypeReference:
		typ := types.Types[n.Token.Text]
		if typ == nil {
			return types.UnknownType(n.Token.Text)
		}
		return typ
	case *ast.ValueExpression:
		switch n.Token.Type {
		case scanner.TokenTypeNumber:
			if n.Token.Value.(int64) > math.MaxInt32 {
				return types.Int64Type
			}
			return types.Int32Type
		case scanner.TokenTypeFloat:
			if n.Token.Value.(float64) > math.MaxFloat32 {
				return types.Float64Type
			}
			return types.Float32Type
		case scanner.TokenTypeString:
			return types.StringType
		case scanner.TokenTypeBoolean:
			return types.BoolType
		default:
			panic(fmt.Errorf("Could not resolve type for token %s", n.Token.String()))
		}
	case *ast.FunctionCall:
		typ := v.getTypeForNode(n.Callee)
		if fnDeclType, ok := typ.(*types.SignatureType); ok {
			return fnDeclType.ReturnType
		}
	case *ast.UnaryExpression:
		return v.getTypeForNode(n.Expression)
	case *ast.BinaryExpression:
		return v.getTypeForNode(n.Left)
	case *ast.FunctionSignature:
		returnType := types.VoidType
		if n.ReturnType != nil {
			returnType = v.getTypeForNode(n.ReturnType)
		}

		argumentsVariables := make([]string, len(n.Arguments))
		for i, arg := range n.Arguments {
			if arg.Name != nil {
				argumentsVariables[i] = arg.Name.Text
			}
		}

		return &types.SignatureType{
			ReturnType:    returnType,
			ArgumentTypes: v.getTypesForNodeList(convertArgumentsToNodes(n.Arguments...)...),
			ArgumentNames: argumentsVariables,
		}
	case *ast.FunctionDeclaration:
		return v.getTypeForNode(n.Signature)
	case *ast.Argument:
		return v.getTypeForNode(n.Type)
	case *ast.ParenExpression:
		return v.getTypeForNode(n.Expression)
	case *ast.TupleDeclaration:
		if n.Type != nil {
			return v.getTypeForNode(n.Type)
		}
		return v.getTypeForNode(n.DefaultValue)
	case *ast.TupleExpression:
		return &types.TupleType{Types: v.getTypesForNodeList(convertExpressionsToNodes(n.Expressions...)...)}
	case *ast.TupleType:
		return &types.TupleType{Types: v.getTypesForNodeList(convertTypesToNodes(n.Types...)...)}
	case *ast.Identifier:
		var tp types.Type = types.UnknownType("undefined")
		v.scopeMustGet(n, func(node ScopeItem) {
			switch n := node.(type) {
			case ast.Node:
				tp = v.getTypeForNode(n)
			}
		})
		return tp
	case *CustomTypeResolvingScopeItem:
		return n.ResolvedType
	default:
		panic(fmt.Errorf("Could not resolve type for %s", reflect.TypeOf(node)))
	}

	return types.UnknownType("undefined")
}

func (v *visitor) getParentFuncDecl() *ast.FunctionDeclaration {
	parent := v.parent
	for parent != nil {
		if funDecl, ok := parent.node.(*ast.FunctionDeclaration); ok {
			return funDecl
		}
	}

	return nil
}

func (v *visitor) isEqualType(a ast.Node, b ast.Node) (bool, types.Type, types.Type) {
	aType := v.getTypeForNode(a)
	bType := v.getTypeForNode(b)
	if aType == nil || bType == nil {
		return false, aType, bType
	}

	return aType.IsEqual(bType), aType, bType
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
typeCheck:
	switch n := node.(type) {
	case *ast.Identifier:
		if n == nil {
			// TODO figure out why we come here
			break
		}

		if v.parent != nil {
			switch v.node.(type) {
			case ast.Declaration, *ast.CallArgument:
				break typeCheck
			}
		}

		scopeItem := v.scope.Get(n.Text, true)
		if scopeItem == nil {
			v.emitError(n, fmt.Sprintf("undefined: %s", n), true)
			break
		}

		v.scope.MarkUsage(scopeItem, n)
	case *ast.FunctionCall:
		funcType := v.getTypeForNode(n.Callee)
		if signType, ok := funcType.(*types.SignatureType); !ok {
			v.emitError(
				n,
				fmt.Sprintf("%s (type %s) is not a function", n.Callee, funcType.GetName()),
				true)
			break
		} else {
			usedArgs := map[string]bool{}
			namedArgs := false
			for i, callArg := range n.Arguments {
				if callArg.Name != nil {
					namedArgs = true
					foundArg := false
					for x, argName := range signType.ArgumentNames {
						if argName == callArg.Name.Text {
							i = x
							foundArg = true
							break
						}
					}

					if !foundArg {
						v.emitError(
							callArg,
							fmt.Sprintf("called function has no argument named %s", callArg.Name.Text),
							true)
						continue
					}
				} else if namedArgs {
					v.emitError(
						n,
						"named and non-named call arguments cannot be mixed",
						true)
				}

				if len(signType.ArgumentNames) > i {
					argName := signType.ArgumentNames[i]
					if _, ok := usedArgs[argName]; ok {
						v.emitError(
							callArg,
							fmt.Sprintf("argument %s already defined", argName),
							true)
					}

					usedArgs[argName] = true

					fnArgType := signType.ArgumentTypes[i]
					exprType := v.getTypeForNode(callArg.Expression)
					equal := fnArgType.IsEqual(exprType)

					if !equal {
						v.emitError(callArg.Expression, fmt.Sprintf(
							"cannot use %s (type %s) as type %s in function call",
							callArg.Expression,
							exprType.GetName(),
							fnArgType.GetName(),
						), true)
					}
				}
			}

			if !namedArgs {
				if len(n.Arguments) < len(signType.ArgumentTypes) {
					v.emitError(n, fmt.Sprintf(
						"too few arguments in call to %s",
						n.Callee,
					), true)
				} else if len(n.Arguments) > len(signType.ArgumentTypes) {
					v.emitError(n, fmt.Sprintf(
						"too many arguments in call to %s",
						n.Callee,
					), true)
				}
			}
		}

	case *ast.ReturnStatement:
		funcDecl := v.getParentFuncDecl()
		funcDeclType := v.getTypeForNode(funcDecl).(*types.SignatureType)

		if n.Expression == nil && funcDeclType.ReturnType == nil {
			break
		}

		returnType := v.getTypeForNode(n.Expression)
		equal := funcDeclType.ReturnType.IsEqual(returnType)

		if !equal {
			v.emitError(n.Expression, fmt.Sprintf(
				"cannot use %s (type %s) as type %s in return statement",
				n.Expression,
				returnType.GetName(),
				funcDeclType.ReturnType.GetName(),
			), true)
			break
		}

	case *ast.BinaryExpression:
		equal, aType, bType := v.isEqualType(n.Left, n.Right)

		if !equal {
			v.emitError(n, fmt.Sprintf(
				"invalid operation: %s (mismatched types %s and %s)",
				n,
				aType.GetName(),
				bType.GetName(),
			), true)
			break
		}
	case *ast.Argument:
		if n.DefaultValue != nil {
			if n.Type != nil {
				equal, aType, bType := v.isEqualType(n, n.DefaultValue)

				if !equal {
					v.emitError(n.DefaultValue, fmt.Sprintf(
						"cannot use %s (type %s) as type %s in assigment",
						n.DefaultValue,
						bType.GetName(),
						aType.GetName(),
					), true)
					break
				}
			}
		}
		if n.Name == nil {
			// TODO figure out why we arrive here
			break
		}
		scopeItem := v.scope.Get(n.Name.Text, false)
		if scopeItem != nil {
			v.emitError(n, fmt.Sprintf("%s already declared", n.Name), true)
			break
		}

		v.scope.Set(n.Name, n)
	case *ast.Block:
		if _, fundeclOk := v.scope.node.(*ast.FunctionDeclaration); fundeclOk {
			break
		}

		return v.subVisitor(node, v.scope.SubScope(node))
	case *ast.FunctionDeclaration:
		if n.Signature.Identifier != nil {
			scopeItem := v.scope.Get(n.Signature.Identifier.Text, false)
			if scopeItem != nil {
				v.emitError(n, fmt.Sprintf("%s already declared", n.Signature.Identifier), true)
				break
			}

			v.scope.Set(n.Signature.Identifier, n)
		}

		return v.subVisitor(node, v.scope.SubScope(node))
	case *ast.TupleDeclaration:
		if n.DefaultValue != nil {
			if n.Type != nil {
				equal, aType, bType := v.isEqualType(n, n.DefaultValue)

				if !equal {
					v.emitError(n.DefaultValue, fmt.Sprintf(
						"cannot use %s (type %s) as type %s in assigment",
						n.DefaultValue,
						bType.GetName(),
						aType.GetName(),
					), true)
					break
				}
			}
		}

		defaultValueType := v.getTypeForNode(n.DefaultValue)
		if defaultValueTupleType, ok := defaultValueType.(*types.TupleType); ok {
			var decl func(patrn *ast.TuplePattern, typ *types.TupleType)
			decl = func(patrn *ast.TuplePattern, typ *types.TupleType) {
				for i, pat := range patrn.Patterns {
					switch p := pat.(type) {
					case *ast.Identifier:
						v.scope.Set(p, &CustomTypeResolvingScopeItem{
							Node:         n,
							ResolvedType: typ.Types[i],
						})
					case *ast.TuplePattern:
						decl(p, typ.Types[i].(*types.TupleType))
					}
				}
			}
			decl(n.Pattern, defaultValueTupleType)
		} else {
			v.emitError(n.DefaultValue, fmt.Sprintf(
				"cannot use %s (type %s) as tuple",
				n.DefaultValue,
				defaultValueType.GetName(),
			), true)
		}
	case *ast.VariableDeclaration:
		if n.DefaultValue != nil {
			if n.Type != nil {
				equal, aType, bType := v.isEqualType(n, n.DefaultValue)

				if !equal {
					v.emitError(n.DefaultValue, fmt.Sprintf(
						"cannot use %s (type %s) as type %s in assigment",
						n.DefaultValue,
						bType.GetName(),
						aType.GetName(),
					), true)
					break
				}
			}
		}

		scopeItem := v.scope.Get(n.Name.Text, false)
		if scopeItem != nil {
			v.emitError(n, fmt.Sprintf("%s already declared", n.Name), true)
			break
		}

		v.scope.Set(n.Name, n)
	case *ast.Assigment:
		equal, leftType, rightType := v.isEqualType(n.Left, n.Right)
		if !equal {
			v.emitError(n.Right, fmt.Sprintf(
				"cannot use %s (type %s) as type %s in assigment expression",
				n.Right,
				rightType.GetName(),
				leftType.GetName(),
			), true)
		}
	}

	return v.subVisitor(node, v.scope)
}

func (v *visitor) Leave(node ast.Node) {

	switch node.(type) {
	case *ast.Block, *ast.FunctionDeclaration:
		unusedScopeItems := v.scope.UnusedScopeItems()
		for _, scopeItemInfo := range unusedScopeItems {
			v.emitError(scopeItemInfo.DefineIdentifier,
				fmt.Sprintf("%s declared but not used", scopeItemInfo.DefineIdentifier.Text),
				false)
		}
	}
}

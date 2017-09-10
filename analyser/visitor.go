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
	types   map[string]ast.Node
	errorCb func(node ast.Node, msg string, fatal bool)
}

func (v *visitor) subVisitor(node ast.Node, scope *Scope) *visitor {
	return &visitor{info: v.info, types: v.types, parent: v, node: node, scope: scope, errorCb: v.errorCb}
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

func (v *visitor) getTypeForTypeName(typName string) types.Type {
	if typ := types.Types[typName]; typ != nil {
		return typ
	}

	if typNode := v.types[typName]; typNode != nil {
		return v.getTypeForNode(typNode)
	}

	return &types.LazyType{Resolver: func() types.Type {
		if typNode := v.types[typName]; typNode != nil {
			return v.getTypeForNode(typNode)
		}

		return types.UnknownType(typName)
	}}
}

func (v *visitor) resolveTypeForNode(node ast.Node) types.Type {
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
	case *ast.ComparisonExpression:
		return types.BoolType
	case *ast.TypeReference:
		return v.getTypeForTypeName(n.Token.Text)
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
		// check if function calls is a typecast
		if ident, ok := n.Callee.(*ast.Identifier); ok {
			typ := v.getType(ident.Text)
			if typ != nil {
				return typ
			}
		}

		typ := v.getTypeForNode(n.Callee)
		if fnDeclType, ok := typ.(*types.SignatureType); ok {
			return fnDeclType.ReturnType
		}
	case *ast.UnaryExpression:
		return v.getTypeForNode(n.Expression)
	case *ast.BinaryExpression:
		leftType := v.getTypeForNode(n.Left)
		rightType := v.getTypeForNode(n.Right)
		leftType, rightType = types.LazyResolve(leftType), types.LazyResolve(rightType)

		var operatorOverload *ast.FunctionDeclaration

	leftRight:
		for _, typ := range []types.Type{leftType, rightType} {
			if structType, ok := typ.(*types.StructType); ok {
				for i, fun := range structType.Functions {
					if fun.Name == n.Operator.Text {
						if fun.Type.ArgumentTypes[0].IsEqual(leftType) && fun.Type.ArgumentTypes[1].IsEqual(rightType) {
							operatorOverload = v.types[structType.Name].(*ast.Struct).Functions[i]
							break leftRight
						}
					}
				}
			}
		}

		if operatorOverload == nil {
			operatorOverload = v.scope.GetOperatorOverload(n.Operator.Text, leftType, rightType)
		}

		if operatorOverload != nil {
			// Overloads should not be recursive
			parentFunc := v.getParentFuncDecl()
			if parentFunc != operatorOverload {
				return v.getTypeForNode(operatorOverload).(*types.SignatureType).ReturnType
			}
		}

		return leftType
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
		// Handle this refs
		if n.Text == "this" {
			structParent := v.getParentStructDecl()
			if structParent != nil {
				return v.getTypeForNode(structParent)
			}
		}

		var tp types.Type = types.UnknownType("undefined")
		v.scopeMustGet(n, func(node ScopeItem) {
			switch n := node.(type) {
			case ast.Node:
				tp = v.getTypeForNode(n)
			}
		})
		return tp
	case *ast.StructExpression:
		return v.getTypeForTypeName(n.Identifier.Text)
	case *ast.Struct:
		typ := &types.StructType{}
		if n.Name != nil {
			typ.Name = n.Name.Text
		}

		for _, varDecl := range n.Variables {
			typ.Variables = append(typ.Variables, struct {
				Name string
				Type types.Type
			}{varDecl.Name.Text, v.getTypeForNode(varDecl)})
		}

		for _, fun := range n.Functions {
			var name string
			if fun.Signature.Identifier != nil {
				name = fun.Signature.Identifier.Text
			} else if fun.Signature.Operator != nil {
				name = fun.Signature.Operator.Text
			}
			typ.Functions = append(typ.Functions, struct {
				Name string
				Type *types.SignatureType
			}{name, v.getTypeForNode(fun).(*types.SignatureType)})
		}

		return typ
	case *ast.Interface:
		typ := &types.InterfaceType{}
		if n.Name != nil {
			typ.Name = n.Name.Text
		}

		for _, signature := range n.Functions {
			var name string
			if signature.Identifier != nil {
				name = signature.Identifier.Text
			} else if signature.Operator != nil {
				name = signature.Operator.Text
			}
			typ.Functions = append(typ.Functions, struct {
				Name string
				Type *types.SignatureType
			}{name, v.getTypeForNode(signature).(*types.SignatureType)})
		}

		return typ
	case *ast.MemberExpression:
		targetType := v.getTypeForNode(n.Target)
		if typeWithMembersType, ok := targetType.(types.TypeWithMembers); ok {
			if ok, typ := typeWithMembersType.HasMember(n.Property.Text); ok {
				return typ
			}
		}

		v.emitError(n, fmt.Sprintf(
			"%s undefined: (type %s has no field or method %s)",
			n,
			targetType.GetName(),
			n.Property.Text,
		), true)
	case *CustomTypeResolvingScopeItem:
		return n.ResolvedType
	default:
		panic(fmt.Errorf("Could not resolve type for %s", reflect.TypeOf(node)))
	}

	return types.UnknownType("undefined")
}

func (v *visitor) getNodeInfo(node ast.Node) *NodeInfo {
	if nodeInfo, ok := v.info.NodeInfo[node]; !ok {
		nodeInfo = &NodeInfo{}
		v.info.NodeInfo[node] = nodeInfo
		return nodeInfo
	} else {
		return nodeInfo
	}
}

func (v *visitor) getTypeForNode(node ast.Node) types.Type {
	nodeInfo := v.getNodeInfo(node)
	if nodeInfo.Type != nil {
		return nodeInfo.Type
	}

	typ := v.resolveTypeForNode(node)
	nodeInfo.Type = typ

	return typ
}

func (v *visitor) getType(typeName string) types.Type {
	// TODO handle custom types
	return types.Types[typeName]
}

func (v *visitor) getParentFuncDecl() *ast.FunctionDeclaration {
	parent := v.parent
	for parent != nil {
		if funDecl, ok := parent.node.(*ast.FunctionDeclaration); ok {
			return funDecl
		}
		return parent.getParentFuncDecl()
	}

	return nil
}

func (v *visitor) getParentStructDecl() *ast.Struct {

	if structDecl, ok := v.node.(*ast.Struct); ok {
		return structDecl
	}
	if v.parent != nil {
		return v.parent.getParentStructDecl()
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

func (v *visitor) validateTypeConversion(call *ast.FunctionCall) bool {
	if ident, ok := call.Callee.(*ast.Identifier); ok {
		typ := v.getType(ident.Text)
		if typ != nil {
			argLen := len(call.Arguments)
			if argLen != 1 {
				if argLen > 1 {
					v.emitError(
						call,
						fmt.Sprintf("too many argument to conversion to %s", typ.GetName()),
						true)
				} else {
					v.emitError(
						call,
						fmt.Sprintf("too few argument to conversion to %s", typ.GetName()),
						true)
				}
				return false
			}

			// TODO check for a named call argument
			expr := call.Arguments[0].Expression
			exprType := v.getTypeForNode(expr)

			if exprType.IsEqual(typ) {
				return true
			}

			conversionOk := false
			switch exprType {
			case types.Float32Type, types.Float64Type, types.Int32Type, types.Int64Type:
				switch typ {
				case types.Float32Type, types.Float64Type, types.Int32Type, types.Int64Type:
					conversionOk = true
				}
			}

			if !conversionOk {
				// cannot convert "" (type string) to type int
				v.emitError(
					call,
					fmt.Sprintf(
						"cannot convert %s (%s) to type %s",
						expr,
						exprType.GetName(),
						typ.GetName(),
					),
					true)
			}

		}
	}
	return true
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	nodeInfo := v.getNodeInfo(node)
	nodeInfo.Scope = v.scope
	nodeInfo.Node = node
	nodeInfo.Parent = v.getNodeInfo(v.node)
	nodeInfo.Parent.Children = append(nodeInfo.Parent.Children, nodeInfo)

typeCheck:
	switch n := node.(type) {
	case *ast.Identifier:
		if n == nil {
			// TODO figure out why we come here
			break
		}

		switch n := v.node.(type) {
		case *ast.TupleDeclaration:
			// Just continue as normal
		case *ast.CallArgument:
			// Identifier is call argument name
			if n.Name == node {
				break typeCheck
			}
		case *ast.FunctionCall:
			// check if call is a typecast
			if ident, ok := n.Callee.(*ast.Identifier); ok {
				typ := v.getType(ident.Text)
				if typ != nil {
					break typeCheck
				}
			}
		case ast.Declaration, *ast.StructExpression, *ast.Struct, *ast.Interface:
			break typeCheck
		}

		if n.Text == "this" {
			structParent := v.getParentStructDecl()
			if structParent != nil {
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
		// Check if function call is a typecast
		if ident, ok := n.Callee.(*ast.Identifier); ok {
			typ := v.getType(ident.Text)
			if typ != nil {
				v.validateTypeConversion(n)
				nodeInfo.TypeCast = true
				break
			}
		}

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

	case *ast.StructExpression:
		identType := types.LazyResolve(v.getTypeForTypeName(n.Identifier.Text))
		if structType, structTypeOk := identType.(*types.StructType); !structTypeOk {
			v.emitError(
				n,
				fmt.Sprintf("%s (type %s) is not a struct", n.Identifier, identType.GetName()),
				true)
			break
		} else {
			usedArgs := map[string]bool{}
			namedArgs := false
			for i, callArg := range n.Arguments {
				if callArg.Name != nil {
					namedArgs = true
					foundArg := false

					for x, vr := range structType.Variables {
						if vr.Name == callArg.Name.Text {
							i = x
							foundArg = true
							break
						}
					}

					if !foundArg {
						v.emitError(
							callArg,
							fmt.Sprintf("struct has no property named %s", callArg.Name.Text),
							true)
						continue
					}
				} else if namedArgs {
					v.emitError(
						n,
						"named and non-named properties cannot be mixed",
						true)
				}
				if len(structType.Variables) > i {
					argName := structType.Variables[i].Name
					if _, ok := usedArgs[argName]; ok {
						v.emitError(
							callArg,
							fmt.Sprintf("property %s already defined", argName),
							true)
					}

					usedArgs[argName] = true

					structArgType := structType.Variables[i].Type
					exprType := v.getTypeForNode(callArg.Expression)
					equal := structArgType.IsEqual(exprType)

					if !equal {
						v.emitError(callArg.Expression, fmt.Sprintf(
							"cannot use %s (type %s) as type %s in struct initializer",
							callArg.Expression,
							exprType.GetName(),
							structArgType.GetName(),
						), true)
					}
				}
			}

			if !namedArgs && len(n.Arguments) != 0 {
				if len(n.Arguments) < len(structType.Variables) {
					v.emitError(n, fmt.Sprintf(
						"too few properties for %s",
						n.Identifier,
					), true)
				} else if len(n.Arguments) > len(structType.Variables) {
					v.emitError(n, fmt.Sprintf(
						"too many properties for %s",
						n.Identifier,
					), true)
				}
			}
		}

	case *ast.ReturnStatement:
		funcDecl := v.getParentFuncDecl()
		funcDeclType := v.getTypeForNode(funcDecl).(*types.SignatureType)

		if n.Expression == nil && (funcDeclType.ReturnType == nil || funcDeclType.ReturnType == types.VoidType) {
			break
		}

		if n.Expression == nil {
			v.emitError(n, fmt.Sprintf(
				"missing return value with type %s",
				funcDeclType.ReturnType.GetName(),
			), true)
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
		aType, bType = types.LazyResolve(aType), types.LazyResolve(bType)

		var operatorOverload *ast.FunctionDeclaration

	leftRight:
		for _, typ := range []types.Type{aType, bType} {
			if structType, ok := typ.(*types.StructType); ok {
				for i, fun := range structType.Functions {
					if fun.Name == n.Operator.Text {
						if fun.Type.ArgumentTypes[0].IsEqual(aType) && fun.Type.ArgumentTypes[1].IsEqual(bType) {
							operatorOverload = v.types[structType.Name].(*ast.Struct).Functions[i]
							break leftRight
						}
					}
				}
			}
		}

		if operatorOverload == nil {
			operatorOverload = v.scope.GetOperatorOverload(n.Operator.Text, aType, bType)
		}

		if operatorOverload != nil {
			// Check that overload is not recursive
			parentFunc := v.getParentFuncDecl()
			if parentFunc != operatorOverload {
				nodeInfo.OverloadedOperation = operatorOverload
				break
			}
		}

		if !equal {
			v.emitError(n, fmt.Sprintf(
				"invalid operation: %s (mismatched types %s and %s)",
				n,
				aType.GetName(),
				bType.GetName(),
			), true)
		}

	case *ast.ComparisonExpression:
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
		if _, fundeclOk := v.node.(*ast.FunctionDeclaration); fundeclOk {
			break
		}

		return v.subVisitor(node, v.scope.SubScope(node))
	case *ast.FunctionDeclaration:
		// Struct member function dont need to be added to scope
		structParen, structParentOk := v.node.(*ast.Struct)

		if n.Signature.Identifier != nil {
			scopeItem := v.scope.Get(n.Signature.Identifier.Text, false)
			if scopeItem != nil {
				v.emitError(n, fmt.Sprintf("%s already declared", n.Signature.Identifier), true)
				break
			}

			if structParentOk {
				break
			}

			v.scope.Set(n.Signature.Identifier, n)
		} else if n.Signature.Operator != nil {
			argCount := len(n.Signature.Arguments)
			if argCount != 2 {
				if argCount < 2 {
					v.emitError(n, "too few arguments for an operator overload", true)
				} else {
					v.emitError(n, "too many arguments for an operator overload", true)
				}
			} else {
				// Add operator overload
				typs := v.getTypesForNodeList(convertArgumentsToNodes(n.Signature.Arguments...)...)

				if structParentOk {
					structParenType := v.getTypeForNode(structParen)
					if !typs[0].IsEqual(structParenType) && !typs[1].IsEqual(structParenType) {
						v.emitError(
							n,
							fmt.Sprintf("Other one of the arguments needs to match type %s", structParenType.GetName()),
							true)
					}
				} else {
					v.scope.SetOperatorOverload(n.Signature.Operator.Text, typs[0], typs[1], n)
				}
			}
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

		// Struct properties dont need to be added to scope
		if _, structParentOk := v.node.(*ast.Struct); structParentOk {
			break
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
	case *ast.Struct:
		nodeInfo.Type = v.getTypeForNode(node)
		// TODO check that it is not redeclared
		// TODO check that no property or function is double declared
		if n.Name != nil {
			v.types[n.Name.Text] = n
		}
	case *ast.Interface:
		// TODO check that it is not redeclared
		// TODO check that no property or function is double declared
		nodeInfo.Type = v.getTypeForNode(node)
		if n.Name != nil {
			v.types[n.Name.Text] = n
		}
	case *ast.MemberExpression:
		nodeInfo.Type = v.getTypeForNode(node)
		targetType := v.getTypeForNode(n.Target)
		if typeWithMembersType, ok := targetType.(types.TypeWithMembers); ok {
			if ok, _ := typeWithMembersType.HasMember(n.Property.Text); ok {
				break
			}
		}

		v.emitError(n, fmt.Sprintf(
			"%s undefined: (type %s has no field or method %s)",
			n,
			targetType.GetName(),
			n.Property.Text,
		), true)
	}

	return v.subVisitor(node, v.scope)
}

func (v *visitor) isMainFuncion(info ScopeItem) bool {
	if _, ok := v.node.(*ast.File); ok {
		if funcDecl, ok := info.(*ast.FunctionDeclaration); ok {
			if funcDecl.Signature.Identifier != nil {
				return funcDecl.Signature.Identifier.Text == "main"
			}
		}
	}
	return false
}

func (v *visitor) Leave(node ast.Node) {
	switch node.(type) {
	case *ast.Block, *ast.File:
		unusedScopeItems := v.scope.UnusedScopeItems()
		for _, scopeItemInfo := range unusedScopeItems {
			if v.isMainFuncion(scopeItemInfo.ScopeItem) {
				break
			}

			v.emitError(scopeItemInfo.DefineIdentifier,
				fmt.Sprintf("%s declared but not used", scopeItemInfo.DefineIdentifier.Text),
				false)

		}
	}
}

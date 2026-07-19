package analyser

import (
	"fmt"
	"math"
	"reflect"
	"strings"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/cheader"
	"github.com/orktes/orlang/scanner"
	"github.com/orktes/orlang/types"
)

type visitor struct {
	node           ast.Node
	scope          *Scope
	info           *FileInfo
	parent         *visitor
	errorCb        func(node ast.Node, msg string, fatal bool)
	autocompleteCb func([]AutoCompleteInfo)
	fileLoader     func(path string) (*ast.File, error)
}

func (v *visitor) subVisitor(node ast.Node, scope *Scope) *visitor {
	return &visitor{
		info:           v.info,
		parent:         v,
		node:           node,
		scope:          scope,
		errorCb:        v.errorCb,
		autocompleteCb: v.autocompleteCb,
		fileLoader:     v.fileLoader,
	}
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

	if typNode := v.info.Types[typName]; typNode != nil {
		resolvedType := v.getTypeForNode(typNode)
		return resolvedType
	}

	return &types.LazyType{Resolver: func() types.Type {
		if typNode := v.info.Types[typName]; typNode != nil {
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
	case *ast.MapType:
		return &types.MapType{
			KeyType:   v.getTypeForNode(n.KeyType),
			ValueType: v.getTypeForNode(n.ValueType),
		}
	case *ast.ChannelType:
		return &types.ChannelType{
			Elem: v.getTypeForNode(n.Type),
		}
	case *ast.ArrayExpression:
		length := int64(len(n.Expressions))
		if n.Type.Length != nil {
			if valExpr, ok := n.Type.Length.(*ast.ValueExpression); ok {
				if valExpr.Token.Type == scanner.TokenTypeNumber {
					length = valExpr.Token.Value.(int64)
				}
			}
		}
		return &types.ArrayType{
			Type:   v.getTypeForNode(n.Type.Type),
			Length: length,
		}
	case *ast.MapExpression:
		return &types.MapType{
			KeyType:   v.getTypeForNode(n.Type.KeyType),
			ValueType: v.getTypeForNode(n.Type.ValueType),
		}
	case *ast.VariableDeclaration:
		if n.Type != nil {
			return v.getTypeForNode(n.Type)
		}
		return v.getTypeForNode(n.DefaultValue)
	case *ast.ComparisonExpression:
		return types.BoolType
	case *ast.TypeReference:
		return v.getTypeForTypeName(n.Name.Text)
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
			v.emitError(n, fmt.Sprintf("could not resolve type for token %s", n.Token.String()), true)
			return types.UnknownType("unresolved")
		}
	case *ast.FunctionCall:
		// check if function calls is a typecast
		if ident, ok := n.Callee.(*ast.Identifier); ok {
			typ := v.getType(ident.Text)
			if typ != nil {
				return typ
			}
			// Builtin functions
			if ident.Text == "str" {
				return types.PrimitiveType{Type: "string"}
			}
			if ident.Text == "len" {
				return types.Int32Type
			}
			if ident.Text == "contains" {
				return types.BoolType
			}
			if ident.Text == "print" || ident.Text == "println" || ident.Text == "delete" {
				return types.VoidType
			}
			if ident.Text == "channel" {
				return &types.ChannelType{}
			}
			if ident.Text == "recv" {
				if len(n.Arguments) == 1 {
					if ch, ok := types.LazyResolve(v.getTypeForNode(n.Arguments[0].Expression)).(*types.ChannelType); ok && ch.Elem != nil {
						return ch.Elem
					}
				}
				return types.UnknownType("recv")
			}
			if ident.Text == "closed" {
				return types.BoolType
			}
			if ident.Text == "send" || ident.Text == "close" || ident.Text == "yield" {
				return types.VoidType
			}
			if ident.Text == "append" {
				// append returns a dynamic slice of the input's element type
				if len(n.Arguments) > 0 {
					if arrType, ok := types.LazyResolve(v.getTypeForNode(n.Arguments[0].Expression)).(*types.ArrayType); ok {
						return &types.ArrayType{Type: arrType.Type, Length: -1}
					}
				}
				return types.UnknownType("append")
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
							operatorOverload = v.info.Types[structType.Name].(*ast.Struct).Functions[i]
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

		// Mixed numeric operands take the unified (wider / non-literal) type.
		if ok, unified := numericOperandsCompatible(n.Left, n.Right, leftType, rightType); ok {
			return unified
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
	case *ast.Enum:
		enumType := &types.EnumType{
			Name:   n.Name.Text,
			Values: make(map[string]int32),
		}
		for i, val := range n.Values {
			enumType.Values[val.Name.Text] = int32(i)
		}
		return enumType
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
	case *ast.IndexExpression:
		// Get the type of the target (should be an array, map, or string)
		targetType := v.getTypeForNode(n.Target)
		if arrayType, ok := targetType.(*types.ArrayType); ok {
			// Return the element type
			return arrayType.Type
		}
		if mapType, ok := targetType.(*types.MapType); ok {
			return mapType.ValueType
		}
		if targetType != nil && targetType.GetName() == "string" {
			// Indexing a string yields the byte at that position
			return types.UInt8Type
		}

		v.emitError(n, fmt.Sprintf(
			"invalid operation: %s (type %s does not support indexing)",
			n,
			targetType.GetName(),
		), true)
		return types.UnknownType("cannot index")
	case *CustomTypeResolvingScopeItem:
		return n.ResolvedType
	case *ast.PointerType:
		// Resolve the pointed-to type
		pointedType := v.getTypeForNode(n.Type)
		return &types.PointerType{Type: pointedType}
	case *ast.TypeAssertionExpression:
		// Type assertions always return bool
		return types.BoolType
	case *ast.CastExpression:
		return v.getTypeForNode(n.Type)
	default:
		// Previously this panicked, crashing the whole compiler; a fatal
		// diagnostic keeps the process alive and points at the location.
		v.emitError(node, fmt.Sprintf("internal: cannot resolve type for %s", reflect.TypeOf(n).String()), true)
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

	// Break self-referential resolution cycles (e.g. a struct whose method
	// signatures mention the struct itself) with a lazy reference to the
	// eventually-resolved type.
	if v.info.resolving[node] {
		return &types.LazyType{Resolver: func() types.Type {
			if info := v.info.NodeInfo[node]; info != nil && info.Type != nil {
				return info.Type
			}
			return types.UnknownType("recursive type")
		}}
	}
	v.info.resolving[node] = true
	typ := v.resolveTypeForNode(node)
	delete(v.info.resolving, node)
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
		parent = parent.parent
	}

	return nil
}

func (v *visitor) getParentForLoop() ast.Node {
	parent := v.parent
	for parent != nil {
		switch parent.node.(type) {
		case *ast.ForLoop, *ast.ForRangeLoop:
			return parent.node
		}
		parent = parent.parent
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
			case types.Float32Type, types.Float64Type, types.Int32Type, types.Int64Type, types.Int16Type, types.Int8Type, types.UInt64Type, types.UInt32Type, types.UInt16Type, types.UInt8Type:
				switch typ {
				case types.Float32Type, types.Float64Type, types.Int32Type, types.Int64Type, types.Int16Type, types.Int8Type, types.UInt64Type, types.UInt32Type, types.UInt16Type, types.UInt8Type:
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

func (v *visitor) checkAutoComplete(ident *ast.Identifier) {

	if ident.Value == nil {
		// Ident doesnt have an autocomplete marker
		return
	}

	value, valueOk := ident.Value.(string)
	if !valueOk || !strings.Contains(value, "#") {
		// Ident doesnt have an autocomplete marker
		return
	}

	prefix := value[:strings.Index(value, "#")]
	keys := []AutoCompleteInfo{}

	switch parent := v.node.(type) {
	case *ast.MemberExpression:
		targetType := v.getTypeForNode(parent.Target)
		if targeType, targeTypeOk := targetType.(types.TypeWithMembers); targeTypeOk {
			members := targeType.GetMembers()
			for _, mem := range members {
				if strings.HasPrefix(mem.Name, prefix) {
					var kind = "Property"
					if _, ok := mem.Type.(*types.SignatureType); ok {
						kind = "Method"
					}

					keys = append(keys, AutoCompleteInfo{
						Label: mem.Name,
						Type:  mem.Type,
						Kind:  kind,
					})
				}
			}
		}
	case *ast.TypeReference:
		for key, typ := range types.Types {
			if strings.HasPrefix(key, prefix) {
				keys = append(keys, AutoCompleteInfo{
					Label: key,
					Type:  typ,
					Kind:  "Reference",
				})
			}
		}
		for key, typ := range v.info.Types {
			if strings.HasPrefix(key, prefix) {
				keys = append(keys, AutoCompleteInfo{
					Label: key,
					Type:  v.getTypeForNode(typ),
					Kind:  "Reference",
				})
			}
		}
	default:
		structParent := v.getParentStructDecl()
		if structParent != nil {
			keys = append(keys, AutoCompleteInfo{
				Label: "this",
				Type:  v.getTypeForNode(structParent),
				Kind:  "Variable",
			})
		}

		v.scope.Traverse(true, func(key string, scopeItem *ScopeItemDetails) {
			if strings.HasPrefix(key, prefix) {
				typ := v.getTypeForNode(scopeItem.ScopeItem)
				kind := "Variable"
				keys = append(keys, AutoCompleteInfo{
					Label: key,
					Type:  typ,
					Kind:  kind,
				})
			}
		})

		for key, typ := range types.Types {
			if strings.HasPrefix(key, prefix) {
				keys = append(keys, AutoCompleteInfo{
					Label: key,
					Type:  typ,
					Kind:  "Reference",
				})
			}
		}
		for key, typ := range v.info.Types {
			if strings.HasPrefix(key, prefix) {
				keys = append(keys, AutoCompleteInfo{
					Label: key,
					Type:  v.getTypeForNode(typ),
					Kind:  "Class",
				})
			}
		}
	}

	v.autocompleteCb(keys)
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

		if v.autocompleteCb != nil {
			v.checkAutoComplete(n)
		}

		switch n := v.node.(type) {
		case *ast.TupleDeclaration:
			// Just continue as normal
		case *ast.CallArgument:
			// Identifier is call argument name
			if n.Name == node {
				break typeCheck
			}
		case *ast.MemberExpression:
			if n.Property == node {
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
		case ast.Declaration, *ast.StructExpression, *ast.Struct, *ast.Interface, *ast.TypeReference:
			shouldCheck := false
			if varDecl, ok := v.node.(*ast.VariableDeclaration); ok {
				if varDecl.DefaultValue == node {
					shouldCheck = true
				}
			}

			if !shouldCheck {
				break typeCheck
			}
		}

		if n.Text == "this" {
			structParent := v.getParentStructDecl()
			if structParent != nil {
				break typeCheck
			}
		}

		scopeItem := v.scope.Get(n.Text, true)
		if scopeItem == nil {
			// Skip error for builtin functions
			switch n.Text {
			case "len", "append", "str", "print", "println", "delete", "contains",
				"channel", "send", "recv", "close", "closed", "yield":
				break
			default:
				v.emitError(n, fmt.Sprintf("undefined: %s", n), true)
			}
			break
		}

		v.scope.MarkUsage(scopeItem, n)

		if details := v.scope.GetDetails(n.Text, true); details != nil {
			if _, ok := details.ScopeItem.(*ast.VariableDeclaration); ok {
				if !details.Initialized {
					v.emitError(n, fmt.Sprintf("variable %s used before initialized", n), false)
				}
			}
		}
	case *ast.FunctionCall:
		// Check for builtin functions
		if ident, ok := n.Callee.(*ast.Identifier); ok && ident.Text == "len" {
			if len(n.Arguments) != 1 {
				v.emitError(n, "len() takes exactly one argument", true)
				break
			}
			arg := n.Arguments[0]
			v.Visit(arg.Expression)

			// Check argument type
			argType := types.LazyResolve(v.getTypeForNode(arg.Expression))
			_, isArray := argType.(*types.ArrayType)
			_, isMap := argType.(*types.MapType)
			if !isArray && !isMap && argType.GetName() != "string" {
				v.emitError(arg.Expression, "len() argument must be array, map, or string", true)
			}

			// Set return type to int32
			nodeInfo.Type = types.Int32Type
			break
		}

		if ident, ok := n.Callee.(*ast.Identifier); ok && (ident.Text == "print" || ident.Text == "println") &&
			v.scope.Get(ident.Text, true) == nil {
			for _, arg := range n.Arguments {
				v.Visit(arg.Expression)
				argType := types.LazyResolve(v.getTypeForNode(arg.Expression))
				switch {
				case argType == nil:
				case argType.GetName() == "string" || argType.GetName() == "bool":
				default:
					if _, numeric := numericKinds[argType.GetName()]; !numeric {
						v.emitError(arg.Expression, fmt.Sprintf(
							"%s() cannot print value of type %s",
							ident.Text,
							argType.GetName(),
						), true)
					}
				}
			}
			nodeInfo.Type = types.VoidType
			break
		}

		if ident, ok := n.Callee.(*ast.Identifier); ok && (ident.Text == "delete" || ident.Text == "contains") &&
			v.scope.Get(ident.Text, true) == nil {
			if len(n.Arguments) != 2 {
				v.emitError(n, fmt.Sprintf("%s() takes exactly two arguments (map, key)", ident.Text), true)
				break
			}
			mapArg := n.Arguments[0]
			keyArg := n.Arguments[1]
			v.Visit(mapArg.Expression)
			v.Visit(keyArg.Expression)

			mapType, isMap := types.LazyResolve(v.getTypeForNode(mapArg.Expression)).(*types.MapType)
			if !isMap {
				v.emitError(mapArg.Expression, fmt.Sprintf("%s() first argument must be a map", ident.Text), true)
				break
			}
			keyType := types.LazyResolve(v.getTypeForNode(keyArg.Expression))
			if keyType != nil && !keyType.IsEqual(mapType.KeyType) {
				v.emitError(keyArg.Expression, fmt.Sprintf(
					"cannot use %s (type %s) as type %s map key",
					keyArg.Expression,
					keyType.GetName(),
					mapType.KeyType.GetName(),
				), true)
			}

			if ident.Text == "contains" {
				nodeInfo.Type = types.BoolType
			} else {
				nodeInfo.Type = types.VoidType
			}
			break
		}

		if ident, ok := n.Callee.(*ast.Identifier); ok && ident.Text == "channel" &&
			v.scope.Get(ident.Text, true) == nil {
			if len(n.Arguments) != 1 {
				v.emitError(n, "channel() takes exactly one argument (capacity)", true)
				break
			}
			v.Visit(n.Arguments[0].Expression)
			capType := types.LazyResolve(v.getTypeForNode(n.Arguments[0].Expression))
			if capType != nil && !strings.HasPrefix(capType.GetName(), "int") && !strings.HasPrefix(capType.GetName(), "uint") {
				v.emitError(n.Arguments[0].Expression, "channel() capacity must be an integer", true)
			}
			nodeInfo.Type = &types.ChannelType{}
			break
		}

		if ident, ok := n.Callee.(*ast.Identifier); ok && ident.Text == "send" &&
			v.scope.Get(ident.Text, true) == nil {
			if len(n.Arguments) != 2 {
				v.emitError(n, "send() takes exactly two arguments (channel, value)", true)
				break
			}
			v.Visit(n.Arguments[0].Expression)
			v.Visit(n.Arguments[1].Expression)
			ch, isChan := types.LazyResolve(v.getTypeForNode(n.Arguments[0].Expression)).(*types.ChannelType)
			if !isChan {
				v.emitError(n.Arguments[0].Expression, "send() first argument must be a channel", true)
				break
			}
			valType := types.LazyResolve(v.getTypeForNode(n.Arguments[1].Expression))
			if ch.Elem != nil && valType != nil && !valType.IsEqual(ch.Elem) &&
				!isAssignable(n.Arguments[1].Expression, valType, ch.Elem) {
				v.emitError(n.Arguments[1].Expression, fmt.Sprintf(
					"cannot send %s (type %s) on channel of %s",
					n.Arguments[1].Expression,
					valType.GetName(),
					ch.Elem.GetName(),
				), true)
			}
			nodeInfo.Type = types.VoidType
			break
		}

		if ident, ok := n.Callee.(*ast.Identifier); ok &&
			(ident.Text == "recv" || ident.Text == "close" || ident.Text == "closed") &&
			v.scope.Get(ident.Text, true) == nil {
			if len(n.Arguments) != 1 {
				v.emitError(n, fmt.Sprintf("%s() takes exactly one argument (channel)", ident.Text), true)
				break
			}
			v.Visit(n.Arguments[0].Expression)
			ch, isChan := types.LazyResolve(v.getTypeForNode(n.Arguments[0].Expression)).(*types.ChannelType)
			if !isChan {
				v.emitError(n.Arguments[0].Expression, fmt.Sprintf("%s() argument must be a channel", ident.Text), true)
				break
			}
			switch ident.Text {
			case "recv":
				if ch.Elem == nil {
					v.emitError(n.Arguments[0].Expression, "cannot receive from an untyped channel; annotate the channel declaration", true)
					break
				}
				nodeInfo.Type = ch.Elem
			case "closed":
				nodeInfo.Type = types.BoolType
			default:
				nodeInfo.Type = types.VoidType
			}
			break
		}

		if ident, ok := n.Callee.(*ast.Identifier); ok && ident.Text == "yield" &&
			v.scope.Get(ident.Text, true) == nil {
			if len(n.Arguments) != 0 {
				v.emitError(n, "yield() takes no arguments", true)
				break
			}
			nodeInfo.Type = types.VoidType
			break
		}

		if ident, ok := n.Callee.(*ast.Identifier); ok && ident.Text == "str" {
			if len(n.Arguments) != 1 {
				v.emitError(n, "str() takes exactly one argument", true)
				break
			}
			arg := n.Arguments[0]
			v.Visit(arg.Expression)

			// Set return type to string
			nodeInfo.Type = types.PrimitiveType{Type: "string"}
			break
		}

		if ident, ok := n.Callee.(*ast.Identifier); ok && ident.Text == "append" {
			if len(n.Arguments) < 2 {
				v.emitError(n, "append() requires at least 2 arguments", true)
				break
			}

			// First argument is the slice
			sliceArg := n.Arguments[0]
			v.Visit(sliceArg.Expression)

			// Check argument type
			sliceType := v.getTypeForNode(sliceArg.Expression)
			arrayType, isArray := sliceType.(*types.ArrayType)
			if !isArray {
				v.emitError(sliceArg.Expression, "append() first argument must be a slice", true)
				break
			}

			// Visit remaining arguments and check they match element type
			for i := 1; i < len(n.Arguments); i++ {
				v.Visit(n.Arguments[i].Expression)
				argType := v.getTypeForNode(n.Arguments[i].Expression)
				if !argType.IsEqual(arrayType.Type) &&
					!isAssignable(n.Arguments[i].Expression, argType, arrayType.Type) {
					v.emitError(n.Arguments[i].Expression, fmt.Sprintf("append() argument type mismatch: expected %s, got %s", arrayType.Type.GetName(), argType.GetName()), true)
				}
			}

			// Return type is a dynamic slice (not fixed array)
			// Create a new ArrayType with Length=-1 to indicate slice
			nodeInfo.Type = &types.ArrayType{
				Type:   arrayType.Type,
				Length: -1, // Dynamic slice
			}
			break
		}

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
					if argName != "" {
						if _, ok := usedArgs[argName]; ok {
							v.emitError(
								callArg,
								fmt.Sprintf("argument %s already defined", argName),
								true)
						}
						usedArgs[argName] = true
					}

					fnArgType := signType.ArgumentTypes[i]
					exprType := v.getTypeForNode(callArg.Expression)
					equal := fnArgType.IsEqual(exprType)

					// Allow &int8 to be passed as string (C-style strings)
					if !equal {
						if ptrType, ok := exprType.(*types.PointerType); ok {
							if primitive, ok := ptrType.Type.(types.PrimitiveType); ok {
								if primitive.Type == "int8" && fnArgType.GetName() == "string" {
									equal = true
								}
							}
						}
					}

					// Allow string to be passed as &int8
					if !equal {
						if ptrType, ok := fnArgType.(*types.PointerType); ok {
							if primitive, ok := ptrType.Type.(types.PrimitiveType); ok {
								if primitive.Type == "int8" && exprType.GetName() == "string" {
									equal = true
								}
							}
						}
					}

					// Allow safe numeric widening and in-range literals
					if !equal && isAssignable(callArg.Expression, exprType, fnArgType) {
						equal = true
					}

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
				// Check if function is variadic by looking at the last argument
				isVariadic := false
				var funDecl *ast.FunctionDeclaration
				if ident, ok := n.Callee.(*ast.Identifier); ok {
					if item := v.scope.Get(ident.Text, true); item != nil {
						funDecl, _ = item.(*ast.FunctionDeclaration)
					}
				}
				if funDecl != nil && len(funDecl.Signature.Arguments) > 0 {
					lastArg := funDecl.Signature.Arguments[len(funDecl.Signature.Arguments)-1]
					isVariadic = lastArg.Variadic
				}

				minArgs := len(signType.ArgumentTypes)
				if isVariadic {
					minArgs-- // Variadic parameter is optional
				}

				if len(n.Arguments) < minArgs {
					v.emitError(n, fmt.Sprintf(
						"too few arguments in call to %s",
						n.Callee,
					), true)
				} else if !isVariadic && len(n.Arguments) > len(signType.ArgumentTypes) {
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
					equal := structArgType.IsEqual(exprType) ||
						isAssignable(callArg.Expression, exprType, structArgType)

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
		equal := funcDeclType.ReturnType.IsEqual(returnType) ||
			isAssignable(n.Expression, returnType, funcDeclType.ReturnType)

		if !equal {
			v.emitError(n.Expression, fmt.Sprintf(
				"cannot use %s (type %s) as type %s in return statement",
				n.Expression,
				returnType.GetName(),
				funcDeclType.ReturnType.GetName(),
			), true)
			break
		}

	case *ast.BreakStatement:
		if v.getParentForLoop() == nil {
			v.emitError(n, "break outside of loop", true)
		}

	case *ast.ContinueStatement:
		if v.getParentForLoop() == nil {
			v.emitError(n, "continue outside of loop", true)
		}

	case *ast.SwitchStatement:
		// Walk is handled by ast.Walk; just validate here
		break

	case *ast.DeferStatement:
		// Walk is handled by ast.Walk
		break

	case *ast.IfStatement:
		v.checkBoolCondition(n.Condition)

	case *ast.ForLoop:
		v.checkBoolCondition(n.Condition)
		// Create a subscope so init variables (e.g., var i = 0) don't leak
		// into the parent scope. This prevents the bug where reusing the same
		// variable name across multiple for-loops references the wrong alloca.
		return v.subVisitor(node, v.scope.SubScope(node))

	case *ast.ForRangeLoop:
		// Create a subscope for iteration variables
		loopScope := v.scope.SubScope(node)
		// Register iteration variables in the new scope
		iterableType := types.LazyResolve(v.getTypeForNode(n.Iterable))
		if arrType, ok := iterableType.(*types.ArrayType); ok {
			// Register value variable: scope item is ForRangeLoop, type is element type
			loopScope.Set(n.ValueName, n)
			v.info.NodeInfo[n.ValueName] = &NodeInfo{Type: arrType.Type}
			// Set type on ForRangeLoop itself so getTypeForNode returns element type
			v.getNodeInfo(n).Type = arrType.Type
			// Register index variable with int32 type
			if n.IndexName != nil {
				loopScope.Set(n.IndexName, n.IndexName)
				v.info.NodeInfo[n.IndexName] = &NodeInfo{Type: types.Int32Type}
			}
		} else if mapType, ok := iterableType.(*types.MapType); ok {
			if n.IndexName != nil {
				// for var k, v in m — first variable is the key, second the value
				loopScope.Set(n.IndexName, n.IndexName)
				v.info.NodeInfo[n.IndexName] = &NodeInfo{Type: mapType.KeyType}
				loopScope.Set(n.ValueName, n)
				v.info.NodeInfo[n.ValueName] = &NodeInfo{Type: mapType.ValueType}
				v.getNodeInfo(n).Type = mapType.ValueType
			} else {
				// for var k in m — iterates over the keys
				loopScope.Set(n.ValueName, n)
				v.info.NodeInfo[n.ValueName] = &NodeInfo{Type: mapType.KeyType}
				v.getNodeInfo(n).Type = mapType.KeyType
			}
		} else if iterableType != nil {
			if _, unknown := iterableType.(types.UnknownType); !unknown {
				v.emitError(n.Iterable, fmt.Sprintf(
					"cannot range over %s (type %s)",
					n.Iterable,
					iterableType.GetName(),
				), true)
			}
		}
		return v.subVisitor(node, loopScope)

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
							operatorOverload = v.info.Types[structType.Name].(*ast.Struct).Functions[i]
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
			// Allow string + int (substring offset)
			if n.Operator.Text == "+" && aType.GetName() == "string" {
				if bName := bType.GetName(); bName == "int32" || bName == "int64" {
					break
				}
			}
			if ok, _ := numericOperandsCompatible(n.Left, n.Right, aType, bType); ok {
				break
			}
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
			if n.Operator.Text != "&&" && n.Operator.Text != "||" {
				if ok, _ := numericOperandsCompatible(n.Left, n.Right, aType, bType); ok {
					break
				}
			}
			v.emitError(n, fmt.Sprintf(
				"invalid operation: %s (mismatched types %s and %s)",
				n,
				aType.GetName(),
				bType.GetName(),
			), true)
			break
		}
	case *ast.CastExpression:
		v.getTypeForNode(n.Left)
	case *ast.Argument:
		if n.DefaultValue != nil {
			if n.Type != nil {
				equal, aType, bType := v.isEqualType(n, n.DefaultValue)

				if !equal && !isAssignable(n.DefaultValue, bType, aType) {
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
	case *ast.ImportStatement:
		path := n.Path.Token.Value.(string)

		if v.fileLoader == nil {
			v.emitError(n, "imports not supported (no file loader)", true)
			break
		}

		importedFile, err := v.fileLoader(path)
		if err != nil {
			v.emitError(n, fmt.Sprintf("failed to load import: %s", err), true)
			break
		}

		importedAnalyser, err := New(importedFile)
		if err != nil {
			v.emitError(n, fmt.Sprintf("failed to create analyser for import: %s", err), true)
			break
		}
		importedAnalyser.FileLoader = v.fileLoader

		importedInfo, err := importedAnalyser.Analyse()
		if err != nil {
			v.emitError(n, fmt.Sprintf("failed to analyse import: %s", err), true)
			break
		}

		// Make the imported module's type declarations resolvable here even
		// when not explicitly imported: an imported function may mention
		// them in its signature (e.g. server() => Server). Locally declared
		// names take precedence.
		if importedFileInfo := importedInfo.FileInfo[importedFile]; importedFileInfo != nil {
			for name, typNode := range importedFileInfo.Types {
				if _, exists := v.info.Types[name]; !exists {
					v.info.Types[name] = typNode
				}
			}
		}

		// Import symbols
		importedScope := importedAnalyser.scope
		for _, item := range n.Items {
			importName := item.Name.Text
			localName := importName
			if item.Alias != nil {
				localName = item.Alias.Text
			}

			details := importedScope.GetDetails(importName, true)
			if details == nil {
				v.emitError(item.Name, fmt.Sprintf("symbol %s not found in %s", importName, path), true)
				continue
			}

			if !details.Exported {
				v.emitError(item.Name, fmt.Sprintf("symbol %s is not exported from %s", importName, path), true)
				continue
			}

			// Add to current scope using the alias (or original name if no alias)
			localIdent := item.Name
			if item.Alias != nil {
				localIdent = item.Alias
			}
			v.scope.Set(localIdent, details.ScopeItem)

			// If it is a type, add it to v.info.Types
			if _, ok := details.ScopeItem.(*ast.Struct); ok {
				v.info.Types[localName] = details.ScopeItem
			} else if _, ok := details.ScopeItem.(*ast.Interface); ok {
				v.info.Types[localName] = details.ScopeItem
			}

			// Mark as initialized since it comes from another file
			v.scope.GetDetails(localName, false).Initialized = true

			// Populate NodeInfo
			typ := v.getTypeForNode(details.ScopeItem)
			v.info.NodeInfo[localIdent] = &NodeInfo{Type: typ}
		}

		// Don't descend into the import items: the original symbol names
		// are not identifiers in this file's scope (only aliases are), so
		// walking them would produce bogus "undefined" errors.
		return nil

	case *ast.ExportStatement:
		ast.Walk(v, n.Declaration)

		var name string
		if fn, ok := n.Declaration.(*ast.FunctionDeclaration); ok {
			if fn.Signature.Identifier != nil {
				name = fn.Signature.Identifier.Text
			}
		} else if variable, ok := n.Declaration.(*ast.VariableDeclaration); ok {
			name = variable.Name.Text
		} else if struc, ok := n.Declaration.(*ast.Struct); ok {
			name = struc.Name.Text
		} else if iface, ok := n.Declaration.(*ast.Interface); ok {
			name = iface.Name.Text
		}

		if name != "" {
			details := v.scope.GetDetails(name, false)
			if details != nil {
				details.Exported = true
			}
		}

		return nil

	case *ast.IncludeStatement:
		// Harvest function declarations from the C header so included
		// functions resolve during analysis (codegen declares the same set).
		headerPath := n.Path.Token.Value.(string)
		funcs, err := cheader.ParseFile(headerPath)
		if err != nil {
			v.emitError(n, fmt.Sprintf("cannot read included header %s: %s", headerPath, err), true)
			break
		}
		for _, cfn := range funcs {
			ident := &ast.Identifier{Token: scanner.Token{Text: cfn.Name}}
			sig := &types.SignatureType{
				ReturnType: cTypeToOrlangType(cfn.ReturnType),
			}
			for _, p := range cfn.Params {
				sig.ArgumentNames = append(sig.ArgumentNames, "")
				sig.ArgumentTypes = append(sig.ArgumentTypes, cTypeToOrlangType(p))
			}
			item := &CustomTypeResolvingScopeItem{ResolvedType: sig}
			v.scope.Set(ident, item)
			v.scope.MarkUsage(item, ident)
			if details := v.scope.GetDetails(cfn.Name, false); details != nil {
				details.Initialized = true
			}
		}
		return nil

	case *ast.LinkStatement:
		// Link directives are handled by the compile pipeline
		break

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

			// Struct member functions are not added to the scope, but they
			// still need their own subscope below so parameters of sibling
			// methods don't collide.
			if !structParentOk {
				v.scope.Set(n.Signature.Identifier, n)
			}

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

				if !equal && !isAssignable(n.DefaultValue, bType, aType) {
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
		if n.DefaultValue != nil {
			v.scope.SetInitialized(n.Name.Text, true)
		}
	case *ast.Assigment:
		equal, leftType, rightType := v.isEqualType(n.Left, n.Right)
		if !equal && !isAssignable(n.Right, rightType, leftType) {
			v.emitError(n.Right, fmt.Sprintf(
				"cannot use %s (type %s) as type %s in assigment expression",
				n.Right,
				rightType.GetName(),
				leftType.GetName(),
			), true)
		}

		if ident, ok := n.Left.(*ast.Identifier); ok {
			v.checkConstAssignment(n, ident)
			v.scope.SetInitialized(ident.Text, true)
		}
	case *ast.UnaryExpression:
		// ++ and -- mutate their operand
		if n.Operator.Type == scanner.TokenTypeIncrement || n.Operator.Type == scanner.TokenTypeDecrement {
			if ident, ok := n.Expression.(*ast.Identifier); ok {
				v.checkConstAssignment(n, ident)
			}
		}
	case *ast.Struct:
		nodeInfo.Type = v.getTypeForNode(node)
		if n.Name != nil {
			if _, exists := v.info.Types[n.Name.Text]; exists {
				v.emitError(n, fmt.Sprintf("%s already declared", n.Name.Text), true)
				break
			}
			v.info.Types[n.Name.Text] = n
			v.scope.Set(n.Name, n)
		}
	case *ast.Enum:
		nodeInfo.Type = v.getTypeForNode(node)
		if n.Name != nil {
			v.info.Types[n.Name.Text] = n
			v.scope.Set(n.Name, n)
		}
	case *ast.Interface:
		nodeInfo.Type = v.getTypeForNode(node)
		if n.Name != nil {
			if _, exists := v.info.Types[n.Name.Text]; exists {
				v.emitError(n, fmt.Sprintf("%s already declared", n.Name.Text), true)
				break
			}
			v.info.Types[n.Name.Text] = n
			v.scope.Set(n.Name, n)
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
	case *ast.IndexExpression:
		nodeInfo.Type = v.getTypeForNode(node)
		// Verify target is indexable (array, map, or string)
		targetType := v.getTypeForNode(n.Target)
		_, isArray := targetType.(*types.ArrayType)
		_, isMap := targetType.(*types.MapType)
		isString := targetType != nil && targetType.GetName() == "string"
		if !isArray && !isMap && !isString {
			v.emitError(n, fmt.Sprintf(
				"invalid operation: %s (type %s does not support indexing)",
				n,
				targetType.GetName(),
			), true)
		}
		// Verify index is an integer for arrays (maps allow string keys)
		if !isMap {
			indexType := v.getTypeForNode(n.Index)
			if !strings.HasPrefix(indexType.GetName(), "int") && !strings.HasPrefix(indexType.GetName(), "uint") {
				v.emitError(n, fmt.Sprintf(
					"non-integer index %s (type %s)",
					n.Index,
					indexType.GetName(),
				), true)
			}
		}
	}

	return v.subVisitor(node, v.scope)
}

func (v *visitor) isRootLevel() (ok bool) {
	if v.parent != nil {
		_, ok = v.parent.node.(*ast.File)
	}
	return
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

func (v *visitor) processUnusedVariables() {
	unusedScopeItems := v.scope.UnusedScopeItems()
	for _, scopeItemInfo := range unusedScopeItems {
		if v.isMainFuncion(scopeItemInfo.ScopeItem) {
			break
		}

		// Type declarations are not "unused variables" — they are part of
		// the file's public shape even when nothing references them yet.
		switch scopeItemInfo.ScopeItem.(type) {
		case *ast.Struct, *ast.Interface, *ast.Enum:
			continue
		}

		v.emitError(scopeItemInfo.DefineIdentifier,
			fmt.Sprintf("%s declared but not used", scopeItemInfo.DefineIdentifier.Text),
			false)

	}
}

func (v *visitor) Leave(node ast.Node) {
	switch node.(type) {
	case *ast.Block:
		if !v.isRootLevel() {
			if n, ok := v.node.(*ast.FunctionDeclaration); ok {
				closure := &Closure{
					FunctionDeclaration: n,
				}

				for scopeItem, refs := range v.scope.GetReferencedItems() {
					ref := refs[0]
					definingScope := v.scope.GetDefiningScope(ref.Text)
					if definingScope == nil {
						// Defined in a subscope of this function (e.g. a
						// for-loop variable): local, not a captured reference.
						continue
					}
					if _, ok := definingScope.node.(*ast.File); !ok {
						// Not defined in root scope so reference needed
						closure.Env = append(closure.Env, scopeItem)
					}

					nodeInfo := v.getNodeInfo(scopeItem)
					nodeInfo.Closures = append([]*Closure{closure}, nodeInfo.Closures...)
				}

				v.info.Closures = append([]*Closure{closure}, v.info.Closures...)
			}
		}
		v.processUnusedVariables()
	case *ast.File:
		v.processUnusedVariables()
	}
}

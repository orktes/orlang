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
	if node := v.scope.Get(identifier.Text, true); node == nil {
		v.emitError(identifier, fmt.Sprintf("%s not initialized", identifier), true) // TODO process scope error
	} else {
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
		default:
			panic(fmt.Errorf("Could not resolve type for token %s", n.Token.String()))
		}
	case *ast.FunctionCall:
		typ := v.getTypeForNode(n.Callee)
		if fnDeclType, ok := typ.(*types.SignatureType); ok {
			return fnDeclType.ReturnType
		} else {
			panic("TODO")
			// TODO what to do here
		}
		//return typ
	case *ast.BinaryExpression:
		return v.getTypeForNode(n.Left)
	case *ast.FunctionSignature:
		return &types.SignatureType{
			ReturnType:     v.getTypeForNode(n.ReturnType),
			ArgugmentTypes: v.getTypesForNodeList(convertArgumentsToNodes(n.Arguments...)...),
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
		var tp types.Type
		v.scopeMustGet(n, func(node ScopeItem) {
			switch n := node.(type) {
			case *ast.VariableDeclaration:
				if n.Type != nil {
					tp = v.getTypeForNode(n.Type)
				} else {
					tp = v.getTypeForNode(n.DefaultValue)
				}
			case *ast.FunctionDeclaration:
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

func (v *visitor) isEqualType(a ast.Node, b ast.Node) (bool, types.Type, types.Type) {
	aType := v.getTypeForNode(a)
	bType := v.getTypeForNode(b)
	if aType == nil || bType == nil {
		return false, aType, bType
	}

	return aType.IsEqual(bType), aType, bType
}

func (v *visitor) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.Block:
		return v.subVisitor(node, v.scope.SubScope())

	case *ast.FunctionDeclaration:
		if n.Signature.Identifier != nil {
			scopeItem := v.scope.Get(n.Signature.Identifier.Text, false)
			if scopeItem != nil {
				v.emitError(n, fmt.Sprintf("%s already declared", n.Signature.Identifier), true)
				break
			}

			v.scope.Set(n.Signature.Identifier.Text, n)
		}

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

		var decl func(patrn *ast.TuplePattern, typ *types.TupleType)
		decl = func(patrn *ast.TuplePattern, typ *types.TupleType) {
			for i, pat := range patrn.Patterns {
				switch p := pat.(type) {
				case *ast.Identifier:
					v.scope.Set(p.Text, &CustomTypeResolvingScopeItem{
						Node:         n,
						ResolvedType: typ.Types[i],
					})
				case *ast.TuplePattern:
					decl(p, typ.Types[i].(*types.TupleType))
				}
			}
		}
		decl(n.Pattern, v.getTypeForNode(n).(*types.TupleType))

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

		v.scope.Set(n.Name.Text, n)

	case *ast.Assigment:
		if identifier, ok := n.Left.(*ast.Identifier); ok {
			v.scopeMustGet(identifier, func(leftNode ScopeItem) {
				equal, leftType, rightType := v.isEqualType(leftNode, n.Right)
				if !equal {
					v.emitError(n.Right, fmt.Sprintf(
						"cannot use %s (type %s) as type %s in assigment expression",
						n.Right,
						rightType.GetName(),
						leftType.GetName(),
					), true)
					return
				}
			})

		} else {
			panic("TODO")
		}
	}
	return v.subVisitor(node, v.scope)
}

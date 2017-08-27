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
	scope   *Scope
	node    ast.Node
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

func (v *visitor) scopeMustGet(identifier *ast.Identifier, cb func(info *ScopeInfo)) {
	if scopeInfo, err := v.scope.Get(identifier.Text); err != nil {
		v.emitError(identifier, err.Error(), true) // TODO process scope error
	} else {
		cb(scopeInfo)
	}
}

func (v *visitor) getTypeForNode(node ast.Node) types.Type {
	switch n := node.(type) {
	case *ast.VariableDeclaration:
		if n.Type != nil {
			return v.getTypeForNode(n.Type)
		}
	case *ast.TypeReference:

		switch n.Token.Text {
		case "int32":
			return types.Int32Type
		case "float32":
			return types.Float32Type
		case "int64":
			return types.Int64Type
		case "float64":
			return types.Float64Type
		}
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
		return v.getTypeForNode(n.Callee)
	case *ast.BinaryExpression:
		return v.getTypeForNode(n.Left)
	case *ast.FunctionDeclaration:
		return v.getTypeForNode(n.Signature.ReturnType)
	case *ast.Argument:
		return v.getTypeForNode(n.Type)
	case *ast.Identifier:
		var tp types.Type
		v.scopeMustGet(n, func(info *ScopeInfo) {
			if dec, ok := info.Declaration.(*ast.VariableDeclaration); ok {
				if dec.Type != nil {
					tp = v.getTypeForNode(dec)
					return
				}
			}
			tp = v.getTypeForNode(info.Initialization)

		})
		return tp
	default:
		panic(fmt.Errorf("Could not resolve type for %s", reflect.TypeOf(node)))
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
	switch n := node.(type) {
	case *ast.Block:
		return v.subVisitor(node, v.scope.SubScope())
	case *ast.FunctionDeclaration:
		if err := v.scope.Declaration(n); err != nil {
			v.emitError(node, err.Error(), true) // TODO process scope erro
		}
	case *ast.VariableDeclaration:
		if n.DefaultValue != nil {
			if n.Type != nil {
				equal, _, _ := v.isEqualType(n, n.DefaultValue)

				if !equal {
					v.emitError(n.DefaultValue, "TODO assigment type error", true)
					break
				}
			}
		}

		if err := v.scope.Declaration(n); err != nil {
			v.emitError(node, err.Error(), true) // TODO process scope error
		}
	case *ast.Assigment:
		if identifier, ok := n.Left.(*ast.Identifier); ok {
			v.scopeMustGet(identifier, func(info *ScopeInfo) {
				if info.Initialization != nil {
					equal, aType, bType := v.isEqualType(info.Initialization, n.Right)

					if !equal {
						v.emitError(n.Right, fmt.Sprintf(
							"cannot use %s (type %s) as type %s in assigment expression",
							n.Right,
							bType.GetName(),
							aType.GetName(),
						), true)
						return
					}
				} else {
					info.Initialization = identifier
				}

			})
		} else {
			panic("TODO")
		}
	}
	return v.subVisitor(node, v.scope)
}

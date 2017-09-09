package ast

import (
	"fmt"
	"reflect"
)

type Visitor interface {
	Visit(node Node) (w Visitor)
}

type Leaver interface {
	Leave(node Node)
}

type VisitorFunc func(node Node) (w Visitor)

func (vf VisitorFunc) Visit(node Node) (w Visitor) {
	return vf(node)
}

func Walk(v Visitor, node Node) {
	if node == nil {
		return
	}
	if leaver, ok := v.(Leaver); ok {
		defer leaver.Leave(node)
	}
	if v = v.Visit(node); v == nil {
		return
	}

	switch n := node.(type) {
	case *Argument:
		Walk(v, n.Name)
		Walk(v, n.Type)
		Walk(v, n.DefaultValue)
	case *Assigment:
		Walk(v, n.Left)
		Walk(v, n.Right)
	case *BinaryExpression:
		Walk(v, n.Left)
		Walk(v, n.Right)
	case *Block:
		for _, nb := range n.Body {
			Walk(v, nb)
		}
	case *CallArgument:
		Walk(v, n.Name)
		Walk(v, n.Expression)
	case *ComparisonExpression:
		Walk(v, n.Left)
		Walk(v, n.Right)
	case *Comment:
		// Nothing to do here
	case *File:
		for _, nb := range n.Body {
			Walk(v, nb)
		}
	case *ForLoop:
		Walk(v, n.Init)
		Walk(v, n.Condition)
		Walk(v, n.After)
		Walk(v, n.Block)
	case *FunctionCall:
		Walk(v, n.Callee)
		for _, nb := range n.Arguments {
			Walk(v, nb)
		}
	case *FunctionSignature:
		Walk(v, n.Identifier)
		for _, nb := range n.Arguments {
			Walk(v, nb)
		}
		if n.ReturnType != nil {
			Walk(v, n.ReturnType)
		}
	case *FunctionDeclaration:
		Walk(v, n.Signature)
		if n.Block != nil {
			Walk(v, n.Block)
		}
	case *IfStatement:
		if n.Condition != nil {
			Walk(v, n.Condition)
		}
		if n.Block != nil {
			Walk(v, n.Block)
		}
		if n.Else != nil {
			Walk(v, n.Else)
		}
	case *Macro:
		// TODO macro
	case *MemberExpression:
		Walk(v, n.Target)
	case *TypeReference:
		// Nothing to do
	case *UnaryExpression:
		Walk(v, n.Expression)
	case *VariableDeclaration:
		Walk(v, n.Name)
		Walk(v, n.Type)
		Walk(v, n.DefaultValue)
	case *ParenExpression:
		Walk(v, n.Expression)
	case *ValueExpression:
	case *Identifier:
	case *ReturnStatement:
		Walk(v, n.Expression)
	case *TuplePattern:
		for _, e := range n.Patterns {
			Walk(v, e)
		}
	case *TupleExpression:
		for _, e := range n.Expressions {
			Walk(v, e)
		}
	case *TupleType:
		for _, t := range n.Types {
			Walk(v, t)
		}
	case *TupleDeclaration:
		Walk(v, n.Pattern)
		Walk(v, n.Type)
		Walk(v, n.DefaultValue)
	case *ArrayExpression:
		Walk(v, n.Type)
		for _, e := range n.Expressions {
			Walk(v, e)
		}
	case *ArrayType:
		if n.Length != nil {
			Walk(v, n.Length)
		}
		Walk(v, n.Type)

	case *Struct:
		Walk(v, n.Name)
		for _, vr := range n.Variables {
			Walk(v, vr)
		}
		for _, fn := range n.Functions {
			Walk(v, fn)
		}
	case *StructExpression:
		Walk(v, n.Identifier)
		for _, nb := range n.Arguments {
			Walk(v, nb)
		}
	default:
		panic(fmt.Errorf("Unknown node type: %s", reflect.TypeOf(n)))
	}
}

package ast

type Visitor interface {
	Visit(node Node) (w Visitor)
}

func Walk(v Visitor, node Node) {
	if node == nil {
		return
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
		for _, nb := range n.ReturnTypes {
			Walk(v, nb)
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
	case *MultiVariableDeclaration:
		for _, d := range n.Declarations {
			Walk(v, d)
		}
	case *PrimitiveType:
		// Nothing to do
	case *UnaryExpression:
		Walk(v, n.Expression)
	case *VariableDeclaration:
		Walk(v, n.Name)
		Walk(v, n.Type)
		Walk(v, n.DefaultValue)
	case *ValueExpression:
	default:
		panic("Unknown node type")
	}
}

package analyser

import "github.com/orktes/orlang/ast"

func convertExpressionsToNodes(exprs ...ast.Expression) []ast.Node {
	nodes := make([]ast.Node, len(exprs))

	for i, expr := range exprs {
		nodes[i] = expr
	}

	return nodes
}

func convertArgumentsToNodes(args ...*ast.Argument) []ast.Node {
	nodes := make([]ast.Node, len(args))

	for i, arg := range args {
		nodes[i] = arg
	}

	return nodes
}

func convertTypesToNodes(types ...ast.Type) []ast.Node {
	nodes := make([]ast.Node, len(types))

	for i, typ := range types {
		nodes[i] = typ
	}

	return nodes
}

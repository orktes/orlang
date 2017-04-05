package analyzer

import "github.com/orktes/orlang/ast"

type NodeInfo struct {
}

type FileInfo struct {
	nodeInfo map[ast.Node]*NodeInfo
}

type Info struct {
	fileInfo map[*ast.File]*FileInfo
}

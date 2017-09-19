package analyser

import (
	"github.com/orktes/orlang/types"

	"github.com/orktes/orlang/ast"
)

type NodeInfo struct {
	Type                types.Type
	Node                ast.Node
	Parent              *NodeInfo
	Children            []*NodeInfo
	Scope               *Scope
	TypeCast            bool
	OverloadedOperation *ast.FunctionDeclaration
	Closures            []*Closure
}

type FileInfo struct {
	NodeInfo map[ast.Node]*NodeInfo
	Types    map[string]ast.Node
	Closures []*Closure
}

func NewFileInfo() *FileInfo {
	return &FileInfo{
		NodeInfo: map[ast.Node]*NodeInfo{},
		Types:    map[string]ast.Node{},
		Closures: []*Closure{},
	}
}

type Info struct {
	FileInfo map[*ast.File]*FileInfo
}

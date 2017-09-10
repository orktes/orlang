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
}

type FileInfo struct {
	NodeInfo map[ast.Node]*NodeInfo
}

func NewFileInfo() *FileInfo {
	return &FileInfo{
		NodeInfo: map[ast.Node]*NodeInfo{},
	}
}

type Info struct {
	FileInfo map[*ast.File]*FileInfo
}

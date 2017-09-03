package analyser

import (
	"github.com/orktes/orlang/types"

	"github.com/orktes/orlang/ast"
)

type NodeInfo struct {
	Type   types.Type
	Parent ast.Node
	Scope  *Scope
}

type FileInfo struct {
	nodeInfo map[ast.Node]*NodeInfo
}

func NewFileInfo() *FileInfo {
	return &FileInfo{
		nodeInfo: map[ast.Node]*NodeInfo{},
	}
}

type Info struct {
	fileInfo map[*ast.File]*FileInfo
}

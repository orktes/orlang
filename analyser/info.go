package analyser

import (
	"github.com/orktes/orlang/types"

	"github.com/orktes/orlang/ast"
)

type NodeInfo struct {
	Type     types.Type
	Parent   ast.Node
	Scope    *Scope
	TypeCast bool
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

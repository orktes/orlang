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

	// resolving tracks nodes whose types are currently being resolved so
	// self-referential types (e.g. a struct method returning the struct)
	// resolve lazily instead of recursing forever.
	resolving map[ast.Node]bool
}

func NewFileInfo() *FileInfo {
	return &FileInfo{
		NodeInfo:  map[ast.Node]*NodeInfo{},
		Types:     map[string]ast.Node{},
		Closures:  []*Closure{},
		resolving: map[ast.Node]bool{},
	}
}

type Info struct {
	FileInfo map[*ast.File]*FileInfo
}

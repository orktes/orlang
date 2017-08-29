package analyser

import "github.com/orktes/orlang/ast"

type ScopeItem interface {
	ast.Node
}

type Scope struct {
	parent *Scope
	items  map[string]ScopeItem
}

func NewScope() *Scope {
	return &Scope{
		items: map[string]ScopeItem{},
	}
}

func (s *Scope) SubScope() *Scope {
	scope := NewScope()
	scope.parent = s
	return scope
}

func (s *Scope) Get(indentifier string, parent bool) ast.Node {
	if info, ok := s.items[indentifier]; ok {
		return info
	}
	if s.parent != nil && parent {
		return s.parent.Get(indentifier, parent)
	}
	return nil
}

func (s *Scope) Set(identifier string, node ast.Node) {
	s.items[identifier] = node
}

package analyser

import (
	"github.com/orktes/orlang/types"

	"github.com/orktes/orlang/ast"
)

type ScopeItem interface {
	ast.Node
}

type Scope struct {
	parent *Scope
	items  map[string]ScopeItem
	usage  map[ScopeItem][]*ast.Identifier
	node   ast.Node
}

func NewScope(node ast.Node) *Scope {
	return &Scope{
		node:  node,
		items: map[string]ScopeItem{},
		usage: map[ScopeItem][]*ast.Identifier{},
	}
}

func (s *Scope) SubScope(node ast.Node) *Scope {
	scope := NewScope(node)
	scope.parent = s
	return scope
}

func (s *Scope) MarkUsage(si ScopeItem, ident *ast.Identifier) {
	scope := s.GetDefiningScope(ident.Text)
	if scope == nil {
		return
	}

	usages := scope.usage[si]
	usages = append(usages, ident)
	s.usage[si] = usages
}

func (s *Scope) UnusedScopeItems() (scopeItems []ScopeItem) {
	for _, scopeItem := range s.items {
		if usage, ok := s.usage[scopeItem]; !ok || len(usage) == 0 {
			scopeItems = append(scopeItems, scopeItem)
		}
	}

	return
}

func (s *Scope) GetDefiningScope(indentifier string) *Scope {
	if _, ok := s.items[indentifier]; ok {
		return s
	}
	if s.parent != nil {
		return s.parent.GetDefiningScope(indentifier)
	}
	return nil
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

type CustomTypeResolvingScopeItem struct {
	Node         ast.Node
	ResolvedType types.Type
}

func (c *CustomTypeResolvingScopeItem) StartPos() ast.Position {
	return c.Node.StartPos()
}

func (c *CustomTypeResolvingScopeItem) EndPos() ast.Position {
	return c.Node.EndPos()
}

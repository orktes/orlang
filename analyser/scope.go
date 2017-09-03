package analyser

import (
	"github.com/orktes/orlang/types"

	"github.com/orktes/orlang/ast"
)

type ScopeItem interface {
	ast.Node
}

type ScopeItemDetails struct {
	ScopeItem
	DefineIdentifier *ast.Identifier
}

type Scope struct {
	parent *Scope
	items  map[string]*ScopeItemDetails
	usage  map[ScopeItem][]*ast.Identifier
	node   ast.Node
}

func NewScope(node ast.Node) *Scope {
	return &Scope{
		node:  node,
		items: map[string]*ScopeItemDetails{},
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

	if scope.items[ident.Text].DefineIdentifier != ident {
		usages := scope.usage[si]
		usages = append(usages, ident)
		scope.usage[si] = usages
	}
}

func (s *Scope) UnusedScopeItems() (scopeItems []*ScopeItemDetails) {
	for _, scopeItemInfo := range s.items {
		if usage := s.usage[scopeItemInfo.ScopeItem]; len(usage) == 0 {
			scopeItems = append(scopeItems, scopeItemInfo)
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
		return info.ScopeItem
	}
	if s.parent != nil && parent {
		return s.parent.Get(indentifier, parent)
	}
	return nil
}

func (s *Scope) Set(identifier *ast.Identifier, node ast.Node) {
	s.items[identifier.Text] = &ScopeItemDetails{
		ScopeItem:        node,
		DefineIdentifier: identifier,
	}
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

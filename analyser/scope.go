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

type OperatorOverload struct {
	Operator            string
	LeftType            types.Type
	RightType           types.Type
	FunctionDeclaration *ast.FunctionDeclaration
}

type Scope struct {
	parent            *Scope
	items             map[string]*ScopeItemDetails
	operatorOverloads []OperatorOverload
	usage             map[ScopeItem][]*ast.Identifier
	references        map[ScopeItem][]*ast.Identifier
	node              ast.Node
	subScopes         []*Scope
}

func NewScope(node ast.Node) *Scope {
	return &Scope{
		node:              node,
		items:             map[string]*ScopeItemDetails{},
		usage:             map[ScopeItem][]*ast.Identifier{},
		references:        map[ScopeItem][]*ast.Identifier{},
		operatorOverloads: []OperatorOverload{},
	}
}

func (s *Scope) SubScope(node ast.Node) *Scope {
	scope := NewScope(node)
	scope.parent = s
	s.subScopes = append(s.subScopes, scope)
	return scope
}

func (s *Scope) MarkUsage(si ScopeItem, ident *ast.Identifier) {
	scope := s.GetDefiningScope(ident.Text)
	if scope == nil {
		return
	}

	if s != scope {
		// Not defined in this scope
		references := s.references[si]
		references = append(references, ident)
		s.references[si] = references
	}

	if scope.items[ident.Text].DefineIdentifier != ident {
		usages := scope.usage[si]
		usages = append(usages, ident)
		scope.usage[si] = usages
	}
}

func (s *Scope) GetReferencedItems() (items map[ScopeItem][]*ast.Identifier) {
	items = map[ScopeItem][]*ast.Identifier{}

	for item, refs := range s.references {
		items[item] = append(items[item], refs...)
	}

	for _, subScope := range s.subScopes {
		references := subScope.GetReferencedItems()
		for item, refs := range references {
			items[item] = append(items[item], refs...)
		}
	}

	return
}

func (s *Scope) UnusedScopeItems() (scopeItems []*ScopeItemDetails) {
	for _, scopeItemInfo := range s.items {
		if usage := s.usage[scopeItemInfo.ScopeItem]; len(usage) == 0 {
			scopeItems = append(scopeItems, scopeItemInfo)
		}
	}

	return
}

func (s *Scope) GetScopeItems(parent bool) (scopeItems map[string]*ScopeItemDetails) {
	scopeItems = map[string]*ScopeItemDetails{}

	for key, scopeItemInfo := range s.items {
		scopeItems[key] = scopeItemInfo
	}
	if parent && s.parent != nil {
		parentItems := s.parent.GetScopeItems(true)
		for key, scopeItemInfo := range parentItems {
			if _, ok := scopeItems[key]; !ok {
				scopeItems[key] = scopeItemInfo
			}
		}
	}
	return
}

func (s *Scope) traverseScope(parent bool, foundItems map[string]*ScopeItemDetails, visitor func(key string, info *ScopeItemDetails)) {
	if foundItems == nil {
		foundItems = map[string]*ScopeItemDetails{}
	}

	for key, scopeItemInfo := range s.items {
		foundItems[key] = scopeItemInfo
		visitor(key, scopeItemInfo)
	}
	if parent && s.parent != nil {
		s.parent.traverseScope(parent, foundItems, visitor)
	}
}

func (s *Scope) Traverse(parent bool, visitor func(key string, info *ScopeItemDetails)) {
	s.traverseScope(parent, nil, visitor)
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

func (s *Scope) GetDetails(indentifier string, parent bool) *ScopeItemDetails {
	if info, ok := s.items[indentifier]; ok {
		return info
	}
	if s.parent != nil && parent {
		return s.parent.GetDetails(indentifier, parent)
	}
	return nil
}

func (s *Scope) Set(identifier *ast.Identifier, node ast.Node) {
	s.SetWithName(identifier.Text, identifier, node)
}

func (s *Scope) SetWithName(name string, identifier *ast.Identifier, node ast.Node) {
	s.items[name] = &ScopeItemDetails{
		ScopeItem:        node,
		DefineIdentifier: identifier,
	}
}

func (s *Scope) SetOperatorOverload(operator string, typA types.Type, typB types.Type, funDecl *ast.FunctionDeclaration) {
	s.operatorOverloads = append(s.operatorOverloads, OperatorOverload{
		Operator:            operator,
		LeftType:            typA,
		RightType:           typB,
		FunctionDeclaration: funDecl,
	})
}

func (s *Scope) GetOperatorOverload(operator string, typA types.Type, typB types.Type) *ast.FunctionDeclaration {
	for _, oo := range s.operatorOverloads {
		if oo.LeftType.IsEqual(typA) && oo.RightType.IsEqual(typB) && oo.Operator == operator {
			return oo.FunctionDeclaration
		}
	}

	if s.parent != nil {
		return s.parent.GetOperatorOverload(operator, typA, typB)
	}

	return nil
}

type CustomTypeResolvingScopeItem struct {
	Node         ast.Node
	ResolvedType types.Type
}

func (c *CustomTypeResolvingScopeItem) StartPos() ast.Position {
	if c.Node == nil {
		return ast.Position{}
	}
	return c.Node.StartPos()
}

func (c *CustomTypeResolvingScopeItem) EndPos() ast.Position {
	if c.Node == nil {
		return ast.Position{}
	}
	return c.Node.EndPos()
}

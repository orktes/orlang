package analyser

import (
	"errors"

	"github.com/orktes/orlang/ast"
)

var (
	ErrAlreadyDeclared = errors.New("Identifier declared in scope")
	ErrNotDeclared     = errors.New("Identifier not declared in scope")
)

type ScopeInfo struct {
	Declaration    ast.Declaration
	Initialization ast.Expression
	Constant       bool
	Scope          *Scope
}

type Scope struct {
	parent *Scope
	items  map[string]*ScopeInfo
	block  *ast.Block
}

func NewScope() *Scope {
	return &Scope{
		items: map[string]*ScopeInfo{},
	}
}

func (s *Scope) SubScope() *Scope {
	scope := NewScope()
	scope.parent = s
	return scope
}

func (s *Scope) Get(indentifier string) (*ScopeInfo, error) {
	if info, ok := s.items[indentifier]; ok {
		return info, nil
	}
	if s.parent != nil {
		return s.parent.Get(indentifier)
	}
	return nil, ErrNotDeclared
}

func (s *Scope) Declaration(d ast.Declaration) error {
	name := d.GetIdentifier().Text
	if _, ok := s.items[name]; ok {
		return ErrAlreadyDeclared
	}

	// TODO figure out type

	switch t := d.(type) {
	case *ast.VariableDeclaration:
		s.items[name] = &ScopeInfo{
			Declaration:    d,
			Initialization: t.DefaultValue,
			Constant:       t.Constant,
			Scope:          s,
		}
	case *ast.FunctionDeclaration:
		s.items[name] = &ScopeInfo{
			Declaration:    t,
			Initialization: t,
			Constant:       true,
			Scope:          s,
		}
	}

	return nil
}

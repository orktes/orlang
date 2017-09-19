package analyser

import "github.com/orktes/orlang/ast"

type Closure struct {
	FunctionDeclaration *ast.FunctionDeclaration
	Env                 []ScopeItem
}

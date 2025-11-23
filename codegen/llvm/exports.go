package llvm

import (
	"github.com/orktes/orlang/ast"
)

// isExported checks if a declaration is exported
func (lcg *LLVMCodeGen) isExported(decl ast.Node) bool {
	// Walk up to find if this node is wrapped in an ExportStatement
	// Since we don't have parent pointers, we need to track this differently
	// For now, check if the function is in the list of exported symbols
	// This is a simplified approach - in a real implementation, we'd track this in the analyzer

	// Quick check: if it's in analyzer info's exports, it's exported
	// But we don't have direct access, so we use a heuristic:
	// - Functions in non-main modules that aren't prefixed with '_' are assumed exported
	// - This is a limitation of not having export info in semantic analysis

	// For now, we'll implement a simpler approach:
	// Walk the file's body to find ExportStatements
	return lcg.findInExports(decl)
}

func (lcg *LLVMCodeGen) findInExports(decl ast.Node) bool {
	if lcg.currentFile == nil {
		return false
	}

	for _, node := range lcg.currentFile.Body {
		if exportStmt, ok := node.(*ast.ExportStatement); ok {
			if exportStmt.Declaration == decl {
				return true
			}
		}
	}
	return false
}

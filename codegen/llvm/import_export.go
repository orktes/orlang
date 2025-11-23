package llvm

import (
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
	"github.com/orktes/orlang/ast"
	ortypes "github.com/orktes/orlang/types"
)

func (lcg *LLVMCodeGen) visitExportStatement(n *ast.ExportStatement) {
	ast.Walk(lcg, n.Declaration)
}

func (lcg *LLVMCodeGen) visitImportStatement(n *ast.ImportStatement) {
	// For imported symbols, we just need to declare them (not define)
	// The actual implementation will come from linking with the compiled imported file
	for _, ident := range n.Imports {
		// Look up type info
		nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[ident]
		if nodeInfo == nil {
			continue
		}

		// If it's a function, declare it as external
		if sig, ok := nodeInfo.Type.(*ortypes.SignatureType); ok {
			name := ident.Text

			// Check if already declared
			if _, ok := lcg.functions[name]; ok {
				continue
			}

			// Generate LLVM function type
			returnType := lcg.getLLVMTypeFromSemantic(sig.ReturnType)
			var paramTypes []types.Type
			for _, arg := range sig.ArgumentTypes {
				paramTypes = append(paramTypes, lcg.getLLVMTypeFromSemantic(arg))
			}

			// Declare as external function
			var params []*ir.Param
			for _, t := range paramTypes {
				params = append(params, ir.NewParam("", t))
			}

			fn := lcg.module.NewFunc(name, returnType, params...)
			lcg.functions[name] = fn
		}
	}
}

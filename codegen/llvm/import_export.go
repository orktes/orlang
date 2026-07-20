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
	// For imported symbols, we need to declare them with their mangled names
	// Extract the module name from the import path
	importPath := n.Path.Token.Value.(string)
	// Extract module name (e.g., "lib.or" -> "lib", "./lib.or" -> "lib")
	importModuleName := getModuleNameFromPath(importPath)

	for _, item := range n.Items {
		// Use alias if present, otherwise use the original name
		localIdent := item.Name
		if item.Alias != nil {
			localIdent = item.Alias
		}

		// Look up type info
		nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[localIdent]
		if nodeInfo == nil {
			continue
		}

		// If it's a struct type, register the LLVM type
		if structTyp, ok := ortypes.LazyResolve(nodeInfo.Type).(*ortypes.StructType); ok {
			// Create the LLVM struct type from the semantic type
			if _, exists := lcg.structs[item.Name.Text]; !exists {
				var fields []types.Type
				fieldIndices := make(map[string]int)
				for i, v := range structTyp.Variables {
					fields = append(fields, lcg.getLLVMTypeFromSemantic(v.Type))
					fieldIndices[v.Name] = i
				}
				llvmStructType := types.NewStruct(fields...)
				typeDef := lcg.module.NewTypeDef(item.Name.Text, llvmStructType)
				lcg.structs[item.Name.Text] = typeDef
				lcg.structDefinitions[item.Name.Text] = llvmStructType
				lcg.structFields[item.Name.Text] = fieldIndices
			}
			continue
		}

		// If it's a function, declare it as external
		if sig, ok := ortypes.LazyResolve(nodeInfo.Type).(*ortypes.SignatureType); ok {
			// The LLVM function name should be MANGLED if it's from a non-main module
			// Format: modulename__functionname
			llvmFuncName := item.Name.Text
			if importModuleName != "main" {
				llvmFuncName = importModuleName + "__" + item.Name.Text
			}

			// Check if already declared
			existingFn, alreadyDeclared := lcg.functions[llvmFuncName]
			if alreadyDeclared {
				// Function already declared, just set up local name mapping
				lcg.functions[localIdent.Text] = existingFn
				continue
			}

			// Generate LLVM function type (use getLLVMReturnType to match definition)
			returnType := lcg.getLLVMReturnType(sig.ReturnType)
			var paramTypes []types.Type
			for _, arg := range sig.ArgumentTypes {
				paramTypes = append(paramTypes, lcg.getLLVMTypeFromSemantic(arg))
			}

			// Declare as external function with mangled name
			var params []*ir.Param
			for _, t := range paramTypes {
				params = append(params, ir.NewParam("", t))
			}

			fn := lcg.module.NewFunc(llvmFuncName, returnType, params...)
			lcg.functions[llvmFuncName] = fn

			// Map the local name (or alias) to this function
			lcg.functions[localIdent.Text] = fn
		}
	}
}

func getModuleNameFromPath(path string) string {
	// Remove directory path and extension
	// "lib.or" -> "lib"
	// "./lib.or" -> "lib"
	// "../foo/bar.or" -> "bar"

	// Find last slash
	lastSlash := -1
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			lastSlash = i
			break
		}
	}

	baseName := path[lastSlash+1:]

	// Remove extension
	lastDot := -1
	for i := len(baseName) - 1; i >= 0; i-- {
		if baseName[i] == '.' {
			lastDot = i
			break
		}
	}

	if lastDot > 0 {
		return baseName[:lastDot]
	}
	return baseName
}

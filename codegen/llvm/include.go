package llvm

import (
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/cheader"
)

func (lcg *LLVMCodeGen) visitIncludeStatement(n *ast.IncludeStatement) {
	headerPath := n.Path.Token.Value.(string)

	funcs, err := cheader.ParseFile(headerPath)
	if err != nil {
		// The analyser already reported unreadable headers; nothing to emit.
		return
	}

	for _, cfn := range funcs {
		if _, ok := lcg.functions[cfn.Name]; ok {
			continue
		}

		var params []*ir.Param
		for _, p := range cfn.Params {
			params = append(params, ir.NewParam("", cTypeToLLVMType(p)))
		}

		fn := lcg.module.NewFunc(cfn.Name, cTypeToLLVMType(cfn.ReturnType), params...)
		lcg.functions[cfn.Name] = fn
	}
}

// cTypeToLLVMType maps a canonicalized C type (from package cheader) to its
// LLVM representation.
func cTypeToLLVMType(cType string) types.Type {
	if len(cType) > 0 && cType[len(cType)-1] == '*' {
		// Most C pointers map to i8* in LLVM
		return types.I8Ptr
	}

	switch cType {
	case "int", "unsigned", "unsigned int":
		return types.I32
	case "long", "unsigned long", "int64_t", "uint64_t", "size_t":
		return types.I64
	case "short", "unsigned short", "int16_t", "uint16_t":
		return types.I16
	case "char", "unsigned char", "int8_t", "uint8_t":
		return types.I8
	case "void":
		return types.Void
	case "float":
		return types.Float
	case "double":
		return types.Double
	default:
		// Default to i32 for unknown types
		return types.I32
	}
}

package llvm

import (
	"os/exec"
	"regexp"
	"strings"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
	"github.com/orktes/orlang/ast"
)

func (lcg *LLVMCodeGen) visitIncludeStatement(n *ast.IncludeStatement) {
	// Get the header file path
	headerPath := n.Path.Token.Value.(string)

	// Use clang to parse the header and get function declarations
	// We'll use a simple approach: run clang -E to preprocess, then parse with a regex
	// For a more robust solution, we'd use libclang or cgo

	// For now, let's use a simple regex-based parser
	// This is a simplified implementation - a production one would use proper C parsing
	cmd := exec.Command("clang", "-E", "-P", headerPath)
	output, err := cmd.Output()
	if err != nil {
		// If clang fails, silently continue
		return
	}

	// Parse function declarations from preprocessed output
	// Look for patterns like: type function_name(params);
	funcPattern := regexp.MustCompile(`(\w+)\s+(\w+)\s*\(([^)]*)\)\s*;`)
	matches := funcPattern.FindAllStringSubmatch(string(output), -1)

	for _, match := range matches {
		returnTypeName := match[1]
		funcName := match[2]
		paramsStr := match[3]

		// Map C types to LLVM types
		returnType := cTypeToLLVMType(returnTypeName)

		// Parse parameters
		var params []*ir.Param
		if strings.TrimSpace(paramsStr) != "" && strings.TrimSpace(paramsStr) != "void" {
			paramList := strings.Split(paramsStr, ",")
			for i, paramStr := range paramList {
				// Extract type from parameter (e.g., "const char *s" -> "char*")
				paramType := extractCType(paramStr)
				llvmType := cTypeToLLVMType(paramType)
				params = append(params, ir.NewParam("", llvmType))
				_ = i
			}
		}

		// Check if already declared
		if _, ok := lcg.functions[funcName]; ok {
			continue
		}

		// Declare the function
		fn := lcg.module.NewFunc(funcName, returnType, params...)
		lcg.functions[funcName] = fn
	}
}

func cTypeToLLVMType(cType string) types.Type {
	cType = strings.TrimSpace(cType)

	// Remove const, volatile, etc.
	cType = strings.ReplaceAll(cType, "const", "")
	cType = strings.ReplaceAll(cType, "volatile", "")
	cType = strings.TrimSpace(cType)

	// Check for pointer
	isPointer := strings.Contains(cType, "*")
	if isPointer {
		// Most C pointers map to i8* in LLVM
		return types.I8Ptr
	}

	// Remove spaces
	cType = strings.ReplaceAll(cType, " ", "")

	switch cType {
	case "int":
		return types.I32
	case "long", "int64_t":
		return types.I64
	case "short", "int16_t":
		return types.I16
	case "char", "int8_t":
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

func extractCType(paramStr string) string {
	// Extract type from parameter declaration
	// E.g., "const char *s" -> "char*"

	paramStr = strings.TrimSpace(paramStr)

	// Find the last identifier (parameter name)
	// Everything before it is the type
	lastIdentStart := -1
	inPointer := false

	for i := len(paramStr) - 1; i >= 0; i-- {
		ch := paramStr[i]
		if ch == '*' {
			inPointer = true
			continue
		}
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '_' {
			if !inPointer {
				lastIdentStart = i
			}
		} else if ch == ' ' || ch == '\t' {
			if lastIdentStart >= 0 && !inPointer {
				// Found the start of the parameter name
				return strings.TrimSpace(paramStr[:lastIdentStart])
			}
		}
	}

	// If we can't find a parameter name, return the whole thing
	return paramStr
}

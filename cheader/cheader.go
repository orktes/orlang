// Package cheader extracts function declarations from C header files so
// that `include "foo.h"` can register the functions with both the analyser
// (orlang types) and the code generator (LLVM types).
//
// Parsing is intentionally simple: headers are preprocessed with clang when
// available (falling back to the raw file) and declarations are matched
// with a regex. A production-grade implementation would use libclang.
package cheader

import (
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// Function is a C function declaration harvested from a header.
type Function struct {
	Name       string
	ReturnType string   // canonicalized C type, e.g. "int", "char*"
	Params     []string // canonicalized C parameter types
}

var funcPattern = regexp.MustCompile(`(\w+)\s+\**(\w+)\s*\(([^)]*)\)\s*;`)

// ParseFile preprocesses the header at path and returns the function
// declarations found in it.
func ParseFile(path string) ([]Function, error) {
	out, err := exec.Command("clang", "-E", "-P", path).Output()
	if err != nil {
		// clang unavailable or failed — fall back to the raw header text.
		raw, rerr := os.ReadFile(path)
		if rerr != nil {
			return nil, rerr
		}
		out = raw
	}
	return Parse(string(out)), nil
}

// Parse extracts function declarations from preprocessed C source text.
func Parse(src string) []Function {
	var funcs []Function
	for _, match := range funcPattern.FindAllStringSubmatch(src, -1) {
		fn := Function{
			Name:       match[2],
			ReturnType: canonicalType(match[1]),
		}
		paramsStr := strings.TrimSpace(match[3])
		if paramsStr != "" && paramsStr != "void" {
			for _, p := range strings.Split(paramsStr, ",") {
				fn.Params = append(fn.Params, canonicalType(ExtractType(p)))
			}
		}
		funcs = append(funcs, fn)
	}
	return funcs
}

// canonicalType strips qualifiers and whitespace from a C type, keeping a
// trailing * for pointers (e.g. "const char *" -> "char*").
func canonicalType(cType string) string {
	cType = strings.ReplaceAll(cType, "const", "")
	cType = strings.ReplaceAll(cType, "volatile", "")
	isPointer := strings.Contains(cType, "*")
	cType = strings.ReplaceAll(cType, "*", "")
	cType = strings.Join(strings.Fields(cType), " ")
	if isPointer {
		cType += "*"
	}
	return cType
}

// ExtractType strips the parameter name from a C parameter declaration,
// returning just the type (e.g. "const char *s" -> "const char *").
func ExtractType(paramStr string) string {
	paramStr = strings.TrimSpace(paramStr)

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
				return strings.TrimSpace(paramStr[:lastIdentStart])
			}
		}
	}
	return paramStr
}

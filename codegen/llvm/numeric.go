package llvm

import (
	"fmt"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/orktes/orlang/ast"
	ortypes "github.com/orktes/orlang/types"
)

// semanticType returns the analysed type for a node (lazy types resolved),
// or nil if the analyser produced no info for it.
func (lcg *LLVMCodeGen) semanticType(n ast.Node) ortypes.Type {
	fileInfo := lcg.analyserInfo.FileInfo[lcg.currentFile]
	if fileInfo == nil {
		return nil
	}
	info := fileInfo.NodeInfo[n]
	if info == nil || info.Type == nil {
		return nil
	}
	return ortypes.LazyResolve(info.Type)
}

// isUnsignedType reports whether a semantic type is one of the unsigned
// integer primitives.
func isUnsignedType(t ortypes.Type) bool {
	if t == nil {
		return false
	}
	switch t.GetName() {
	case "uint8", "uint16", "uint32", "uint64":
		return true
	}
	return false
}

// operandsUnsigned reports whether a binary/comparison node should use
// unsigned instruction variants: true when either side is analysed as an
// unsigned integer type.
func (lcg *LLVMCodeGen) operandsUnsigned(left, right ast.Node) bool {
	return isUnsignedType(lcg.semanticType(left)) || isUnsignedType(lcg.semanticType(right))
}

// comparisonPredicates maps an orlang comparison operator to its LLVM
// integer predicates (signed and unsigned) and float predicate (ordered).
var comparisonPredicates = map[string]struct {
	signed   enum.IPred
	unsigned enum.IPred
	float    enum.FPred
}{
	"==": {enum.IPredEQ, enum.IPredEQ, enum.FPredOEQ},
	"!=": {enum.IPredNE, enum.IPredNE, enum.FPredONE},
	"<":  {enum.IPredSLT, enum.IPredULT, enum.FPredOLT},
	"<=": {enum.IPredSLE, enum.IPredULE, enum.FPredOLE},
	">":  {enum.IPredSGT, enum.IPredUGT, enum.FPredOGT},
	">=": {enum.IPredSGE, enum.IPredUGE, enum.FPredOGE},
}

// isFloatLLVMType reports whether an LLVM type is float or double.
func isFloatLLVMType(t types.Type) bool {
	_, ok := t.(*types.FloatType)
	return ok
}

// getOrDeclareStrlen returns the C strlen function declared with its real
// ABI: size_t (i64) return, not i32.
func (lcg *LLVMCodeGen) getOrDeclareStrlen() *ir.Func {
	if fn, ok := lcg.functions["strlen"]; ok {
		return fn
	}
	fn := lcg.module.NewFunc("strlen", types.I64, ir.NewParam("str", types.I8Ptr))
	lcg.functions["strlen"] = fn
	return fn
}

// getOrDeclareMemcpy returns the C memcpy function, declaring it if needed.
func (lcg *LLVMCodeGen) getOrDeclareMemcpy() *ir.Func {
	if fn, ok := lcg.functions["memcpy"]; ok {
		return fn
	}
	fn := lcg.module.NewFunc("memcpy", types.I8Ptr,
		ir.NewParam("dest", types.I8Ptr),
		ir.NewParam("src", types.I8Ptr),
		ir.NewParam("n", types.I64))
	lcg.functions["memcpy"] = fn
	return fn
}

// errorf records a code generation error with source position information
// when available. Codegen keeps going so multiple errors can be reported,
// but the compile driver fails the build if any were recorded.
func (lcg *LLVMCodeGen) errorf(n ast.Node, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	if n != nil {
		pos := n.StartPos()
		msg = fmt.Sprintf("%d:%d: %s", pos.Line+1, pos.Column+1, msg)
	}
	lcg.errors = append(lcg.errors, fmt.Errorf("%s", msg))
}

// Errors returns the errors recorded during Generate, if any.
func (lcg *LLVMCodeGen) Errors() []error {
	return lcg.errors
}

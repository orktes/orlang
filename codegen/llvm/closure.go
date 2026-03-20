package llvm

import (
	"fmt"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/orktes/orlang/ast"
	ortypes "github.com/orktes/orlang/types"
)

// getClosureType returns the LLVM struct type for closure values: { i8*, i8* }
// First field is the function pointer (as i8*), second is the environment pointer.
func (lcg *LLVMCodeGen) getClosureType() *types.StructType {
	return types.NewStruct(types.I8Ptr, types.I8Ptr)
}

// getClosureFuncType returns the LLVM function type for a closure-wrapped function.
// It prepends an i8* env parameter before the user parameters.
func (lcg *LLVMCodeGen) getClosureFuncType(sig *ortypes.SignatureType) *types.FuncType {
	retType := lcg.getLLVMReturnType(sig.ReturnType)
	var paramTypes []types.Type
	paramTypes = append(paramTypes, types.I8Ptr) // env pointer
	for _, argType := range sig.ArgumentTypes {
		paramTypes = append(paramTypes, lcg.getLLVMParamType(argType))
	}
	return types.NewFunc(retType, paramTypes...)
}

// getOrCreateWrapper returns a wrapper function for a named function that adds
// an env i8* parameter (which it ignores), so it can be used as a closure value.
func (lcg *LLVMCodeGen) getOrCreateWrapper(funcName string, fn *ir.Func) *ir.Func {
	if wrapper, ok := lcg.closureWrappers[funcName]; ok {
		return wrapper
	}

	wrapperName := "__closure_wrapper_" + funcName

	var wrapperParams []*ir.Param
	wrapperParams = append(wrapperParams, ir.NewParam("env", types.I8Ptr))
	for i, param := range fn.Params {
		wrapperParams = append(wrapperParams, ir.NewParam(fmt.Sprintf("arg%d", i), param.Type()))
	}

	wrapper := lcg.module.NewFunc(wrapperName, fn.Sig.RetType, wrapperParams...)
	if fn.Sig.Variadic {
		wrapper.Sig.Variadic = true
	}

	block := wrapper.NewBlock("")

	// Forward all args except env to the original function
	var forwardArgs []value.Value
	for i := 1; i < len(wrapperParams); i++ {
		forwardArgs = append(forwardArgs, wrapperParams[i])
	}

	if fn.Sig.RetType.Equal(types.Void) {
		block.NewCall(fn, forwardArgs...)
		block.NewRet(nil)
	} else {
		result := block.NewCall(fn, forwardArgs...)
		block.NewRet(result)
	}

	lcg.closureWrappers[funcName] = wrapper
	return wrapper
}

// createClosureValue builds a closure struct { fn_ptr as i8*, env_ptr } in the current block.
func (lcg *LLVMCodeGen) createClosureValue(fnPtr value.Value, envPtr value.Value) value.Value {
	closureType := lcg.getClosureType()

	fnI8 := lcg.currentBlock.NewBitCast(fnPtr, types.I8Ptr)

	var env value.Value
	if envPtr != nil {
		env = envPtr
	} else {
		env = constant.NewNull(types.I8Ptr)
	}

	var closureVal value.Value = constant.NewStruct(
		closureType,
		constant.NewNull(types.I8Ptr),
		constant.NewNull(types.I8Ptr),
	)
	closureVal = lcg.currentBlock.NewInsertValue(closureVal, fnI8, 0)
	closureVal = lcg.currentBlock.NewInsertValue(closureVal, env, 1)
	return closureVal
}

// getOrDeclareGCInit returns the GC_init function, declaring it if needed.
func (lcg *LLVMCodeGen) getOrDeclareGCInit() *ir.Func {
	if fn, ok := lcg.functions["GC_init"]; ok {
		return fn
	}
	fn := lcg.module.NewFunc("GC_init", types.Void)
	lcg.functions["GC_init"] = fn
	return fn
}

// getOrDeclareGCMalloc returns the GC_malloc function, declaring it if needed.
func (lcg *LLVMCodeGen) getOrDeclareGCMalloc() *ir.Func {
	if fn, ok := lcg.functions["GC_malloc"]; ok {
		return fn
	}
	fn := lcg.module.NewFunc("GC_malloc", types.I8Ptr, ir.NewParam("size", types.I64))
	lcg.functions["GC_malloc"] = fn
	return fn
}

// capturedVar holds information about a variable captured by a closure.
type capturedVar struct {
	defineIdent ast.Node    // The DefineIdentifier AST node
	outerValue  value.Value // The alloca in the outer function
	llvmType    types.Type  // The element type to store in the env struct
}

// captureCollector walks an AST subtree and identifies variables captured from outer scope.
type captureCollector struct {
	lcg            *LLVMCodeGen
	lambdaArgNames map[string]bool
	captures       []capturedVar
	seen           map[ast.Node]bool
}

func (cc *captureCollector) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}

	ident, ok := node.(*ast.Identifier)
	if !ok || ident == nil {
		return cc
	}

	// Skip lambda's own parameters
	if cc.lambdaArgNames[ident.Text] {
		return cc
	}

	nodeInfo := cc.lcg.analyserInfo.FileInfo[cc.lcg.currentFile].NodeInfo[ident]
	if nodeInfo == nil {
		return cc
	}

	details := nodeInfo.Scope.GetDetails(ident.Text, true)
	if details == nil {
		return cc
	}

	if cc.seen[details.DefineIdentifier] {
		return cc
	}

	val, ok := cc.lcg.values[details.DefineIdentifier]
	if !ok {
		return cc
	}

	// Only capture allocas (local variables/parameters from outer scope)
	if _, isInst := val.(ir.Instruction); !isInst {
		return cc
	}
	ptrType, isPtr := val.Type().(*types.PointerType)
	if !isPtr {
		return cc
	}

	cc.seen[details.DefineIdentifier] = true
	cc.captures = append(cc.captures, capturedVar{
		defineIdent: details.DefineIdentifier,
		outerValue:  val,
		llvmType:    ptrType.ElemType,
	})

	return cc
}

// detectCaptures walks the lambda body and returns captured variables from the outer scope.
func (lcg *LLVMCodeGen) detectCaptures(body *ast.Block, lambdaArgNames map[string]bool) []capturedVar {
	collector := &captureCollector{
		lcg:            lcg,
		lambdaArgNames: lambdaArgNames,
		seen:           make(map[ast.Node]bool),
	}
	ast.Walk(collector, body)
	return collector.captures
}

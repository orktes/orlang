package llvm

import (
	"strings"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/orktes/orlang/ast"
	ortypes "github.com/orktes/orlang/types"
)

// getOrDeclarePrintf returns libc printf, declaring it if the program has
// not declared it (via extern or include) already.
func (lcg *LLVMCodeGen) getOrDeclarePrintf() *ir.Func {
	if fn, ok := lcg.functions["printf"]; ok {
		return fn
	}
	fn := lcg.module.NewFunc("printf", types.I32, ir.NewParam("fmt", types.I8Ptr))
	fn.Sig.Variadic = true
	lcg.functions["printf"] = fn
	return fn
}

// getOrDeclareMapRuntime declares one of the map runtime helpers provided
// by the embedded runtime library.
func (lcg *LLVMCodeGen) getOrDeclareMapRuntime(name string) *ir.Func {
	if fn, ok := lcg.functions[name]; ok {
		return fn
	}
	var fn *ir.Func
	switch name {
	case "map_len":
		fn = lcg.module.NewFunc(name, types.I64, ir.NewParam("map", types.I8Ptr))
	case "map_contains":
		fn = lcg.module.NewFunc(name, types.I64,
			ir.NewParam("map", types.I8Ptr),
			ir.NewParam("key", types.I8Ptr))
	case "map_delete":
		fn = lcg.module.NewFunc(name, types.Void,
			ir.NewParam("map", types.I8Ptr),
			ir.NewParam("key", types.I8Ptr))
	case "map_keys":
		fn = lcg.module.NewFunc(name, types.NewPointer(types.I8Ptr),
			ir.NewParam("map", types.I8Ptr))
	default:
		return nil
	}
	lcg.functions[name] = fn
	return fn
}

// getMapGetFn returns (or declares) the map_get function.
func (lcg *LLVMCodeGen) getMapGetFn() *ir.Func {
	if fn, ok := lcg.functions["map_get"]; ok {
		return fn
	}
	fn := lcg.module.NewFunc("map_get", types.I64,
		ir.NewParam("map", types.NewPointer(types.I8)),
		ir.NewParam("key", types.I8Ptr))
	lcg.functions["map_get"] = fn
	return fn
}

// isMapNode reports whether a node's analysed type is a map.
func (lcg *LLVMCodeGen) isMapNode(n ast.Node) bool {
	_, ok := lcg.semanticType(n).(*ortypes.MapType)
	return ok
}

// visitPrintBuiltin lowers print(...)/println(...) to a printf call with a
// format string derived from the argument types. println separates the
// arguments with spaces and appends a newline.
func (lcg *LLVMCodeGen) visitPrintBuiltin(n *ast.FunctionCall, newline bool) {
	var format strings.Builder
	var args []value.Value

	for i, arg := range n.Arguments {
		ast.Walk(lcg, arg.Expression)
		val := lcg.values[arg.Expression]
		if val == nil {
			lcg.errorf(arg.Expression, "cannot print this expression")
			return
		}

		if i > 0 && newline {
			format.WriteString(" ")
		}

		unsigned := isUnsignedType(lcg.semanticType(arg.Expression))
		switch t := val.Type().(type) {
		case *types.FloatType:
			format.WriteString("%g")
			if t.Kind == types.FloatKindFloat {
				val = lcg.currentBlock.NewFPExt(val, types.Double)
			}
		case *types.PointerType:
			// Strings are i8*
			format.WriteString("%s")
		case *types.IntType:
			switch {
			case t.BitSize == 1:
				format.WriteString("%s")
				val = lcg.currentBlock.NewSelect(val,
					lcg.addStringConstant("true"),
					lcg.addStringConstant("false"))
			case t.BitSize == 64 && unsigned:
				format.WriteString("%llu")
			case t.BitSize == 64:
				format.WriteString("%lld")
			case unsigned:
				if t.BitSize < 32 {
					val = lcg.currentBlock.NewZExt(val, types.I32)
				}
				format.WriteString("%u")
			default:
				if t.BitSize < 32 {
					val = lcg.currentBlock.NewSExt(val, types.I32)
				}
				format.WriteString("%d")
			}
		default:
			lcg.errorf(arg.Expression, "cannot print value of this type")
			return
		}
		args = append(args, val)
	}

	if newline {
		format.WriteString("\n")
	}

	callArgs := append([]value.Value{lcg.addStringConstant(format.String())}, args...)
	lcg.currentBlock.NewCall(lcg.getOrDeclarePrintf(), callArgs...)
}

// visitMapDeleteBuiltin lowers delete(m, key).
func (lcg *LLVMCodeGen) visitMapDeleteBuiltin(n *ast.FunctionCall) {
	mapVal, keyVal := lcg.mapBuiltinArgs(n)
	if mapVal == nil || keyVal == nil {
		return
	}
	lcg.currentBlock.NewCall(lcg.getOrDeclareMapRuntime("map_delete"), mapVal, keyVal)
}

// visitMapContainsBuiltin lowers contains(m, key) to a bool.
func (lcg *LLVMCodeGen) visitMapContainsBuiltin(n *ast.FunctionCall) {
	mapVal, keyVal := lcg.mapBuiltinArgs(n)
	if mapVal == nil || keyVal == nil {
		return
	}
	res := lcg.currentBlock.NewCall(lcg.getOrDeclareMapRuntime("map_contains"), mapVal, keyVal)
	lcg.values[n] = lcg.currentBlock.NewICmp(enum.IPredNE, res, constant.NewInt(types.I64, 0))
}

// mapBuiltinArgs evaluates the (map, key) argument pair shared by the
// delete/contains builtins.
func (lcg *LLVMCodeGen) mapBuiltinArgs(n *ast.FunctionCall) (value.Value, value.Value) {
	if len(n.Arguments) != 2 {
		return nil, nil
	}
	ast.Walk(lcg, n.Arguments[0].Expression)
	ast.Walk(lcg, n.Arguments[1].Expression)
	return lcg.values[n.Arguments[0].Expression], lcg.values[n.Arguments[1].Expression]
}

// visitMapRangeLoop lowers `for var k in m` / `for var k, v in m` using the
// runtime's map_keys/map_len helpers.
func (lcg *LLVMCodeGen) visitMapRangeLoop(n *ast.ForRangeLoop, mapVal value.Value, mapType *ortypes.MapType) {
	keys := lcg.currentBlock.NewCall(lcg.getOrDeclareMapRuntime("map_keys"), mapVal)
	count64 := lcg.currentBlock.NewCall(lcg.getOrDeclareMapRuntime("map_len"), mapVal)
	count := lcg.currentBlock.NewTrunc(count64, types.I32)

	idxAlloca := lcg.currentBlock.NewAlloca(types.I32)
	lcg.currentBlock.NewStore(constant.NewInt(types.I32, 0), idxAlloca)

	condBlock := lcg.currentFunc.NewBlock("")
	bodyBlock := lcg.currentFunc.NewBlock("")
	postBlock := lcg.currentFunc.NewBlock("")
	afterBlock := lcg.currentFunc.NewBlock("")

	lcg.currentBlock.NewBr(condBlock)

	lcg.currentBlock = condBlock
	idx := lcg.currentBlock.NewLoad(types.I32, idxAlloca)
	cond := lcg.currentBlock.NewICmp(enum.IPredSLT, idx, count)
	lcg.currentBlock.NewCondBr(cond, bodyBlock, afterBlock)

	lcg.currentBlock = bodyBlock
	bodyIdx := lcg.currentBlock.NewLoad(types.I32, idxAlloca)
	keyPtr := lcg.currentBlock.NewGetElementPtr(types.I8Ptr, keys, bodyIdx)
	keyVal := lcg.currentBlock.NewLoad(types.I8Ptr, keyPtr)

	// With two variables the first is the key and the second the value;
	// with one variable it holds the key.
	if n.IndexName != nil {
		keyAlloca := lcg.currentBlock.NewAlloca(types.I8Ptr)
		lcg.currentBlock.NewStore(keyVal, keyAlloca)
		lcg.values[n.IndexName] = keyAlloca

		valueLLVMType := lcg.getLLVMTypeFromSemantic(mapType.ValueType)
		mapGet := lcg.getMapGetFn()
		raw := lcg.currentBlock.NewCall(mapGet, mapVal, keyVal)
		converted := lcg.convertMapValueFromI64(raw, valueLLVMType)
		valAlloca := lcg.currentBlock.NewAlloca(valueLLVMType)
		lcg.currentBlock.NewStore(converted, valAlloca)
		lcg.values[n.ValueName] = valAlloca
	} else {
		keyAlloca := lcg.currentBlock.NewAlloca(types.I8Ptr)
		lcg.currentBlock.NewStore(keyVal, keyAlloca)
		lcg.values[n.ValueName] = keyAlloca
	}

	lcg.loopExitBlocks = append(lcg.loopExitBlocks, afterBlock)
	lcg.loopPostBlocks = append(lcg.loopPostBlocks, postBlock)

	ast.Walk(lcg, n.Block)

	lcg.loopExitBlocks = lcg.loopExitBlocks[:len(lcg.loopExitBlocks)-1]
	lcg.loopPostBlocks = lcg.loopPostBlocks[:len(lcg.loopPostBlocks)-1]

	if !lcg.isTerminator(lcg.currentBlock.Term) {
		lcg.currentBlock.NewBr(postBlock)
	}

	lcg.currentBlock = postBlock
	postIdx := lcg.currentBlock.NewLoad(types.I32, idxAlloca)
	newIdx := lcg.currentBlock.NewAdd(postIdx, constant.NewInt(types.I32, 1))
	lcg.currentBlock.NewStore(newIdx, idxAlloca)
	lcg.currentBlock.NewBr(condBlock)

	lcg.currentBlock = afterBlock
}

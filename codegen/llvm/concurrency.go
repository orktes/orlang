package llvm

import (
	"fmt"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/orktes/orlang/ast"
	ortypes "github.com/orktes/orlang/types"
)

// getOrDeclareTaskRuntime declares one of the green-thread/channel runtime
// functions provided by the embedded runtime library.
func (lcg *LLVMCodeGen) getOrDeclareTaskRuntime(name string) *ir.Func {
	if fn, ok := lcg.functions[name]; ok {
		return fn
	}
	var fn *ir.Func
	switch name {
	case "task_spawn":
		fn = lcg.module.NewFunc(name, types.Void,
			ir.NewParam("fn", types.I8Ptr),
			ir.NewParam("env", types.I8Ptr))
	case "task_yield":
		fn = lcg.module.NewFunc(name, types.Void)
	case "chan_new":
		fn = lcg.module.NewFunc(name, types.I8Ptr, ir.NewParam("capacity", types.I64))
	case "chan_send":
		fn = lcg.module.NewFunc(name, types.Void,
			ir.NewParam("ch", types.I8Ptr),
			ir.NewParam("value", types.I64))
	case "chan_recv":
		fn = lcg.module.NewFunc(name, types.I64, ir.NewParam("ch", types.I8Ptr))
	case "chan_close":
		fn = lcg.module.NewFunc(name, types.Void, ir.NewParam("ch", types.I8Ptr))
	case "chan_closed":
		fn = lcg.module.NewFunc(name, types.I32, ir.NewParam("ch", types.I8Ptr))
	case "chan_select":
		fn = lcg.module.NewFunc(name, types.I32,
			ir.NewParam("n", types.I64),
			ir.NewParam("chans", types.NewPointer(types.I8Ptr)),
			ir.NewParam("dirs", types.NewPointer(types.I32)),
			ir.NewParam("vals", types.NewPointer(types.I64)),
			ir.NewParam("has_default", types.I32))
	default:
		return nil
	}
	lcg.functions[name] = fn
	return fn
}

// channelElemLLVMType returns the LLVM type of a channel expression's
// element, or nil when unknown.
func (lcg *LLVMCodeGen) channelElemLLVMType(chExpr ast.Node) types.Type {
	if ch, ok := lcg.semanticType(chExpr).(*ortypes.ChannelType); ok && ch.Elem != nil {
		return lcg.getLLVMTypeFromSemantic(ch.Elem)
	}
	return nil
}

// visitChannelNewBuiltin lowers channel(capacity).
func (lcg *LLVMCodeGen) visitChannelNewBuiltin(n *ast.FunctionCall) {
	if len(n.Arguments) != 1 {
		return
	}
	ast.Walk(lcg, n.Arguments[0].Expression)
	capVal := lcg.values[n.Arguments[0].Expression]
	if capVal == nil {
		return
	}
	capVal = lcg.numericConvert(capVal, types.I64, n.Arguments[0].Expression)
	lcg.values[n] = lcg.currentBlock.NewCall(lcg.getOrDeclareTaskRuntime("chan_new"), capVal)
}

// visitChannelSendBuiltin lowers send(ch, value): the value is squeezed
// into the channel's i64 slot like map values.
func (lcg *LLVMCodeGen) visitChannelSendBuiltin(n *ast.FunctionCall) {
	if len(n.Arguments) != 2 {
		return
	}
	ast.Walk(lcg, n.Arguments[0].Expression)
	ast.Walk(lcg, n.Arguments[1].Expression)
	chVal := lcg.values[n.Arguments[0].Expression]
	val := lcg.values[n.Arguments[1].Expression]
	if chVal == nil || val == nil {
		return
	}
	// Convert to the channel's element type first so the receiver's
	// conversion back from i64 sees the expected representation.
	if elem := lcg.channelElemLLVMType(n.Arguments[0].Expression); elem != nil && !val.Type().Equal(elem) {
		val = lcg.numericConvert(val, elem, n.Arguments[1].Expression)
	}
	lcg.currentBlock.NewCall(lcg.getOrDeclareTaskRuntime("chan_send"),
		chVal, lcg.convertMapValueToI64(val))
}

// visitChannelRecvBuiltin lowers recv(ch), converting the i64 slot back to
// the channel's element type.
func (lcg *LLVMCodeGen) visitChannelRecvBuiltin(n *ast.FunctionCall) {
	if len(n.Arguments) != 1 {
		return
	}
	ast.Walk(lcg, n.Arguments[0].Expression)
	chVal := lcg.values[n.Arguments[0].Expression]
	if chVal == nil {
		return
	}
	raw := lcg.currentBlock.NewCall(lcg.getOrDeclareTaskRuntime("chan_recv"), chVal)
	elem := lcg.channelElemLLVMType(n.Arguments[0].Expression)
	if elem == nil {
		lcg.errorf(n, "cannot receive from an untyped channel")
		return
	}
	lcg.values[n] = lcg.convertMapValueFromI64(raw, elem)
}

func (lcg *LLVMCodeGen) visitChannelCloseBuiltin(n *ast.FunctionCall) {
	if len(n.Arguments) != 1 {
		return
	}
	ast.Walk(lcg, n.Arguments[0].Expression)
	if chVal := lcg.values[n.Arguments[0].Expression]; chVal != nil {
		lcg.currentBlock.NewCall(lcg.getOrDeclareTaskRuntime("chan_close"), chVal)
	}
}

func (lcg *LLVMCodeGen) visitChannelClosedBuiltin(n *ast.FunctionCall) {
	if len(n.Arguments) != 1 {
		return
	}
	ast.Walk(lcg, n.Arguments[0].Expression)
	chVal := lcg.values[n.Arguments[0].Expression]
	if chVal == nil {
		return
	}
	res := lcg.currentBlock.NewCall(lcg.getOrDeclareTaskRuntime("chan_closed"), chVal)
	lcg.values[n] = lcg.currentBlock.NewICmp(enum.IPredNE, res, constant.NewInt(types.I32, 0))
}

// visitGoStatement lowers `go f(args...)`: the callee (any function value)
// and the arguments are evaluated NOW, packed into a GC-allocated
// environment, and a per-site thunk unpacks them and performs the call on
// the new green thread.
func (lcg *LLVMCodeGen) visitGoStatement(n *ast.GoStatement) {
	call, ok := n.Call.(*ast.FunctionCall)
	if !ok {
		lcg.errorf(n, "go requires a function call")
		return
	}

	sig, _ := lcg.semanticType(call.Callee).(*ortypes.SignatureType)
	if sig == nil {
		lcg.errorf(n, "go requires a function value (methods are not supported yet; wrap the call in fn () => void { ... })")
		return
	}

	// Evaluate the callee to a closure value { fnptr, env }
	ast.Walk(lcg, call.Callee)
	closureVal := lcg.values[call.Callee]
	if closureVal == nil || !closureVal.Type().Equal(lcg.getClosureType()) {
		lcg.errorf(n, "go requires a function value")
		return
	}

	// Evaluate arguments at spawn time, converted to the parameter types
	var argVals []value.Value
	var argLLVMTypes []types.Type
	for i, arg := range call.Arguments {
		ast.Walk(lcg, arg.Expression)
		val := lcg.values[arg.Expression]
		if val == nil {
			lcg.errorf(arg.Expression, "cannot evaluate go argument")
			return
		}
		if i < len(sig.ArgumentTypes) {
			expected := lcg.getLLVMTypeFromSemantic(sig.ArgumentTypes[i])
			if !val.Type().Equal(expected) {
				if ptr, ok := val.Type().(*types.PointerType); ok && ptr.ElemType.Equal(expected) {
					val = lcg.currentBlock.NewLoad(expected, val)
				} else {
					val = lcg.castIfNeeded(val, lcg.semanticType(arg.Expression), sig.ArgumentTypes[i])
				}
			}
		}
		argVals = append(argVals, val)
		argLLVMTypes = append(argLLVMTypes, val.Type())
	}

	// Environment layout: { fnptr i8*, env i8*, args... }
	envFields := append([]types.Type{types.I8Ptr, types.I8Ptr}, argLLVMTypes...)
	envStructType := types.NewStruct(envFields...)

	mallocFn := lcg.getOrDeclareGCMalloc()
	envRaw := lcg.currentBlock.NewCall(mallocFn,
		constant.NewInt(types.I64, lcg.getSizeOf(envStructType)))
	envTyped := lcg.currentBlock.NewBitCast(envRaw, types.NewPointer(envStructType))

	storeField := func(idx int, val value.Value) {
		ptr := lcg.currentBlock.NewGetElementPtr(envStructType, envTyped,
			constant.NewInt(types.I32, 0), constant.NewInt(types.I32, int64(idx)))
		lcg.currentBlock.NewStore(val, ptr)
	}
	storeField(0, lcg.currentBlock.NewExtractValue(closureVal, 0))
	storeField(1, lcg.currentBlock.NewExtractValue(closureVal, 1))
	for i, val := range argVals {
		storeField(2+i, val)
	}

	// Per-site thunk: void thunk(i8* raw) { unpack; fn(env, args...) }
	thunk := lcg.module.NewFunc(fmt.Sprintf("__go_thunk_%d", lcg.goThunkCounter),
		types.Void, ir.NewParam("raw", types.I8Ptr))
	lcg.goThunkCounter++
	block := thunk.NewBlock("")
	typedEnv := block.NewBitCast(thunk.Params[0], types.NewPointer(envStructType))

	loadField := func(idx int, t types.Type) value.Value {
		ptr := block.NewGetElementPtr(envStructType, typedEnv,
			constant.NewInt(types.I32, 0), constant.NewInt(types.I32, int64(idx)))
		return block.NewLoad(t, ptr)
	}
	fnRaw := loadField(0, types.I8Ptr)
	envPtr := loadField(1, types.I8Ptr)

	closureFuncType := lcg.getClosureFuncType(sig)
	fnPtr := block.NewBitCast(fnRaw, types.NewPointer(closureFuncType))
	callArgs := []value.Value{envPtr}
	for i, t := range argLLVMTypes {
		callArgs = append(callArgs, loadField(2+i, t))
	}
	block.NewCall(fnPtr, callArgs...)
	block.NewRet(nil)

	// Spawn the task
	spawn := lcg.getOrDeclareTaskRuntime("task_spawn")
	thunkPtr := lcg.currentBlock.NewBitCast(thunk, types.I8Ptr)
	lcg.currentBlock.NewCall(spawn, thunkPtr, envRaw)
}

// visitSelectStatement lowers a select statement: channel operands and
// send values are evaluated up front into stack arrays, chan_select picks
// (or waits for) a runnable case, and the fired index dispatches to the
// case blocks. Receive bindings convert the i64 slot back to the element
// type.
func (lcg *LLVMCodeGen) visitSelectStatement(n *ast.SelectStatement) {
	// Split cases into operations (indexed) and an optional default.
	var ops []*ast.SelectCase
	var defaultCase *ast.SelectCase
	for _, c := range n.Cases {
		if c.IsDefault {
			defaultCase = c
		} else {
			ops = append(ops, c)
		}
	}
	if len(ops) == 0 {
		if defaultCase != nil {
			ast.Walk(lcg, defaultCase.Block)
		}
		return
	}

	count := int64(len(ops))
	chansType := types.NewArray(uint64(count), types.I8Ptr)
	dirsType := types.NewArray(uint64(count), types.I32)
	valsType := types.NewArray(uint64(count), types.I64)
	chansAlloca := lcg.currentBlock.NewAlloca(chansType)
	dirsAlloca := lcg.currentBlock.NewAlloca(dirsType)
	valsAlloca := lcg.currentBlock.NewAlloca(valsType)

	zero := constant.NewInt(types.I32, 0)
	slot := func(arr value.Value, arrType types.Type, i int64) value.Value {
		return lcg.currentBlock.NewGetElementPtr(arrType, arr, zero, constant.NewInt(types.I32, i))
	}

	for i, c := range ops {
		ast.Walk(lcg, c.Channel)
		chVal := lcg.values[c.Channel]
		if chVal == nil {
			lcg.errorf(c.Channel, "cannot evaluate select channel")
			return
		}
		lcg.currentBlock.NewStore(chVal, slot(chansAlloca, chansType, int64(i)))

		dir := int64(0)
		var initVal value.Value = constant.NewInt(types.I64, 0)
		if c.IsSend {
			dir = 1
			ast.Walk(lcg, c.Value)
			val := lcg.values[c.Value]
			if val == nil {
				lcg.errorf(c.Value, "cannot evaluate select send value")
				return
			}
			if elem := lcg.channelElemLLVMType(c.Channel); elem != nil && !val.Type().Equal(elem) {
				val = lcg.numericConvert(val, elem, c.Value)
			}
			initVal = lcg.convertMapValueToI64(val)
		}
		lcg.currentBlock.NewStore(constant.NewInt(types.I32, dir), slot(dirsAlloca, dirsType, int64(i)))
		lcg.currentBlock.NewStore(initVal, slot(valsAlloca, valsType, int64(i)))
	}

	hasDefault := int64(0)
	if defaultCase != nil {
		hasDefault = 1
	}
	fired := lcg.currentBlock.NewCall(lcg.getOrDeclareTaskRuntime("chan_select"),
		constant.NewInt(types.I64, count),
		slot(chansAlloca, chansType, 0),
		slot(dirsAlloca, dirsType, 0),
		slot(valsAlloca, valsType, 0),
		constant.NewInt(types.I32, hasDefault))

	mergeBlock := lcg.currentFunc.NewBlock("")

	// Dispatch chain over the fired index
	for i, c := range ops {
		caseBlock := lcg.currentFunc.NewBlock("")
		nextBlock := lcg.currentFunc.NewBlock("")
		cond := lcg.currentBlock.NewICmp(enum.IPredEQ, fired, constant.NewInt(types.I32, int64(i)))
		lcg.currentBlock.NewCondBr(cond, caseBlock, nextBlock)

		lcg.currentBlock = caseBlock
		if !c.IsSend && c.VarName != nil {
			raw := lcg.currentBlock.NewLoad(types.I64, slot(valsAlloca, valsType, int64(i)))
			elem := lcg.channelElemLLVMType(c.Channel)
			if elem == nil {
				elem = types.I64
			}
			converted := lcg.convertMapValueFromI64(raw, elem)
			binding := lcg.currentBlock.NewAlloca(elem)
			lcg.currentBlock.NewStore(converted, binding)
			lcg.values[c.VarName] = binding
		}
		ast.Walk(lcg, c.Block)
		if !lcg.isTerminator(lcg.currentBlock.Term) {
			lcg.currentBlock.NewBr(mergeBlock)
		}

		lcg.currentBlock = nextBlock
	}

	// Remaining index (-1) is the default case, or fall through to merge.
	if defaultCase != nil {
		ast.Walk(lcg, defaultCase.Block)
	}
	if !lcg.isTerminator(lcg.currentBlock.Term) {
		lcg.currentBlock.NewBr(mergeBlock)
	}

	lcg.currentBlock = mergeBlock
}

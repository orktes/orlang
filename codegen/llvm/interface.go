package llvm

import (
	"fmt"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	ortypes "github.com/orktes/orlang/types"
)

// getInterfaceType returns the LLVM type for an interface: { i8*, i8* }
// First element: data pointer (void*)
// Second element: itable pointer (void*)
func (lcg *LLVMCodeGen) getInterfaceType() types.Type {
	return types.NewStruct(types.I8Ptr, types.I8Ptr)
}

// createInterfaceCast creates an interface value from a concrete struct value
func (lcg *LLVMCodeGen) createInterfaceCast(val value.Value, sourceTyp ortypes.Type, targetTyp *ortypes.InterfaceType) value.Value {
	// 1. Create the interface struct
	ifaceType := lcg.getInterfaceType()

	// 2. Cast data pointer to i8*
	// If val is a pointer to struct, cast it.
	// If val is already a pointer, we can cast it.
	// If val is a value (not pointer), we might need to alloca and store it?
	// In Orlang, structs are passed by reference (pointers), so val should be a pointer.

	var dataPtr value.Value
	if val.Type().Equal(types.I8Ptr) {
		dataPtr = val
	} else {
		dataPtr = lcg.currentBlock.NewBitCast(val, types.I8Ptr)
	}

	// 3. Generate or get itable
	itable := lcg.getOrCreateItable(sourceTyp, targetTyp)
	itablePtr := lcg.currentBlock.NewBitCast(itable, types.I8Ptr)

	// 4. Create the interface struct value
	// We can't return a struct value directly in LLVM IR easily as a single SSA value if we want to store it.
	// Usually we treat it as a value.
	// Let's create an undef struct and insert values.

	var ifaceVal value.Value = constant.NewUndef(ifaceType)
	ifaceVal = lcg.currentBlock.NewInsertValue(ifaceVal, dataPtr, 0)
	ifaceVal = lcg.currentBlock.NewInsertValue(ifaceVal, itablePtr, 1)

	return ifaceVal
}

// getOrCreateItable generates the itable for a specific concrete type implementing an interface
func (lcg *LLVMCodeGen) getOrCreateItable(sourceTyp ortypes.Type, targetTyp *ortypes.InterfaceType) value.Value {
	// Name for the itable global
	sourceName := sourceTyp.GetName()
	targetName := targetTyp.GetName()
	itableName := fmt.Sprintf("__itable_%s_to_%s", sourceName, targetName)

	// Check if already exists
	for _, g := range lcg.module.Globals {
		if g.Name() == itableName {
			return g
		}
	}

	// Create itable type: array of function pointers
	// For simplicity, we'll use an array of i8* and cast them when calling.
	// Or we can try to be more specific, but function signatures vary.
	// So [N x i8*] is safest.

	numMethods := len(targetTyp.Functions)
	itableType := types.NewArray(uint64(numMethods), types.I8Ptr)

	var methodPtrs []constant.Constant

	// Find implementation for each interface method
	if structTyp, ok := sourceTyp.(*ortypes.StructType); ok {
		for _, ifaceMethod := range targetTyp.Functions {
			// Find matching method in struct
			// We need to find the mangled name of the struct method
			// The struct method name is StructName_MethodName
			// But wait, the struct method expects *StructType as first arg.
			// The interface method caller will pass i8*.
			// So we need a thunk!

			methodName := ifaceMethod.Name
			structMethodName := structTyp.Name + "_" + methodName

			// Check if we have this function
			fn, ok := lcg.functions[structMethodName]
			if !ok {
				// Should not happen if type checking passed
				// But maybe it's defined in another file?
				// For now assume it exists or panic/error
				panic(fmt.Sprintf("Method %s not found for struct %s", structMethodName, structTyp.Name))
			}

			// Create thunk
			thunk := lcg.createThunk(fn, structTyp, ifaceMethod.Type)
			methodPtrs = append(methodPtrs, constant.NewBitCast(thunk, types.I8Ptr))
		}
	}

	itableConst := constant.NewArray(itableType, methodPtrs...)

	g := lcg.module.NewGlobalDef(itableName, itableConst)
	g.Immutable = true

	return g
}

// createThunk creates a wrapper function that casts the first argument (i8*) to the concrete struct pointer
func (lcg *LLVMCodeGen) createThunk(realFn *ir.Func, structTyp *ortypes.StructType, methodSig *ortypes.SignatureType) *ir.Func {
	thunkName := fmt.Sprintf("%s_thunk", realFn.Name())

	// Check if thunk already exists
	if fn, ok := lcg.functions[thunkName]; ok {
		return fn
	}

	// Thunk signature matches the interface method signature, but with 'this' as i8* (or explicit receiver?)
	// Actually, the interface method signature in 'types' package might not include 'this'.
	// But at LLVM level, we need to pass 'this'.
	// The caller will pass i8* as first arg.

	var params []*ir.Param
	params = append(params, ir.NewParam("this", types.I8Ptr))

	for i, argType := range methodSig.ArgumentTypes {
		params = append(params, ir.NewParam(fmt.Sprintf("arg%d", i), lcg.getLLVMTypeFromSemantic(argType)))
	}

	returnType := lcg.getLLVMTypeFromSemantic(methodSig.ReturnType)

	thunk := lcg.module.NewFunc(thunkName, returnType, params...)

	block := thunk.NewBlock("")

	// Cast 'this' (i8*) to *StructType
	// We need to get the LLVM type for the struct
	// We can look it up in lcg.structs using structTyp.Name
	llvmStructType := lcg.structs[structTyp.Name]
	// llvmStructType is the TypeDef or StructType. We need a pointer to it.
	targetPtrType := types.NewPointer(llvmStructType)

	thisCast := block.NewBitCast(params[0], targetPtrType)

	// Prepare arguments for real function
	var args []value.Value
	args = append(args, thisCast)
	for i := 1; i < len(params); i++ {
		args = append(args, params[i])
	}

	// Call real function
	call := block.NewCall(realFn, args...)

	// Return result
	if returnType.Equal(types.Void) {
		block.NewRet(nil)
	} else {
		block.NewRet(call)
	}

	lcg.functions[thunkName] = thunk
	return thunk
}

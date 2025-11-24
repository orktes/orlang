package llvm

import (
	"github.com/llir/llvm/ir/types"
	"github.com/orktes/orlang/ast"
	ortypes "github.com/orktes/orlang/types"
)

func (lcg *LLVMCodeGen) visitCastExpression(n *ast.CastExpression) {
	// Evaluate the expression being cast
	ast.Walk(lcg, n.Left)
	val := lcg.values[n.Left]

	if val == nil {
		return
	}

	// Get semantic types
	fileInfo := lcg.analyserInfo.FileInfo[lcg.currentFile]
	if fileInfo == nil {
		return
	}

	leftInfo := fileInfo.NodeInfo[n.Left]
	if leftInfo == nil {
		return
	}
	leftType := leftInfo.Type

	// Target type info might not be needed if we use getLLVMType(n.Type)

	// We can get the target LLVM type directly from the AST type node
	targetLLVMType := lcg.getLLVMType(n.Type)

	// Resolve lazy types
	if lazy, ok := leftType.(*ortypes.LazyType); ok {
		leftType = lazy.Resolver()
	}

	// Check if we are casting FROM an interface
	if _, isIface := leftType.(*ortypes.InterfaceType); isIface {
		// Unbox interface
		// Interface is {i8* data, i8* itable}

		// 1. Extract data pointer
		dataPtr := lcg.currentBlock.NewExtractValue(val, 0)

		// 2. Cast data pointer to pointer to target type
		// If target is primitive (e.g. i32), we want *i32
		// If target is struct (e.g. %Struct), we want *%Struct (which is what we use for structs)
		// Wait, structs are passed by pointer. So if target is struct, targetLLVMType is %Struct.
		// We want *%Struct.

		targetPtrType := types.NewPointer(targetLLVMType)
		castPtr := lcg.currentBlock.NewBitCast(dataPtr, targetPtrType)

		// 3. Load value if it's a primitive or pointer-wrapped type
		// If target is struct, we usually pass it by pointer, so maybe we just return the pointer?
		// But in Orlang, if I have `var s Struct = ...`, s is a pointer.
		// So `castPtr` IS the value we want?

		// If target is primitive (e.g. int32), we allocated it on stack.
		// So `castPtr` points to the stack slot. We need to load it.

		if _, isStruct := targetLLVMType.(*types.StructType); isStruct {
			// For structs, we use the pointer directly
			lcg.values[n] = castPtr
		} else if targetLLVMType.Equal(types.I8Ptr) {
			// Special case for i8* (string): it was stored directly in dataPtr
			lcg.values[n] = lcg.currentBlock.NewBitCast(dataPtr, targetLLVMType)
		} else {
			// For primitives (int, float, bool) and other pointers, we need to load the value
			lcg.values[n] = lcg.currentBlock.NewLoad(targetLLVMType, castPtr)
		}
		return
	}

	// Regular cast (fallback to castValue or castIfNeeded)
	// For now, just implement basic casts if needed, or rely on implicit casts?
	// But `as` is explicit.

	// Use castValue helper
	// We need to know if types are signed.
	// For now assume signed for ints? Or check type name.

	// TODO: Better signedness check
	lcg.values[n] = lcg.castValue(val, targetLLVMType, true, true)
}

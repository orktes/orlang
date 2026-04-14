package llvm

import (
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	ortypes "github.com/orktes/orlang/types"
)

// ArrayHelper provides centralized array and slice operations
type ArrayHelper struct {
	lcg *LLVMCodeGen
}

// NewArrayHelper creates a new array helper
func NewArrayHelper(lcg *LLVMCodeGen) *ArrayHelper {
	return &ArrayHelper{lcg: lcg}
}

// CreateSlice creates a slice struct { T*, i32 } from a data pointer and length
func (ah *ArrayHelper) CreateSlice(dataPtr value.Value, length int64, elemType types.Type) value.Value {
	sliceType := types.NewStruct(types.NewPointer(elemType), types.I32)

	// Create slice struct
	var sliceVal value.Value = constant.NewStruct(
		sliceType,
		constant.NewNull(types.NewPointer(elemType)),
		constant.NewInt(types.I32, 0),
	)

	sliceVal = ah.lcg.currentBlock.NewInsertValue(sliceVal, dataPtr, 0)
	lengthVal := constant.NewInt(types.I32, length)
	sliceVal = ah.lcg.currentBlock.NewInsertValue(sliceVal, lengthVal, 1)

	return sliceVal
}

// DecayArray converts a fixed-size array alloca to a pointer to its first element
// Input: [N x T]* (pointer to array)
// Output: T* (pointer to first element)
func (ah *ArrayHelper) DecayArray(arrayAlloca value.Value) value.Value {
	ptrType, ok := arrayAlloca.Type().(*types.PointerType)
	if !ok {
		return arrayAlloca
	}

	arrayType, ok := ptrType.ElemType.(*types.ArrayType)
	if !ok {
		return arrayAlloca
	}

	zero := constant.NewInt(types.I32, 0)
	return ah.lcg.currentBlock.NewGetElementPtr(arrayType, arrayAlloca, zero, zero)
}

// ArrayToSlice converts a fixed-size array to a slice struct
// Input: [N x T]* (pointer to array) or T* (decayed pointer)
// Output: { T*, i32 } (slice struct)
func (ah *ArrayHelper) ArrayToSlice(arrayVal value.Value, arrayType *ortypes.ArrayType) value.Value {
	elemType := ah.lcg.getLLVMTypeFromSemantic(arrayType.Type)

	var dataPtr value.Value
	zero := constant.NewInt(types.I32, 0)

	// Check if arrayVal is already a decayed pointer (T*) or array pointer ([N x T]*)
	if arrayVal.Type().Equal(types.NewPointer(elemType)) {
		// Already decayed
		dataPtr = arrayVal
	} else if ptrType, ok := arrayVal.Type().(*types.PointerType); ok {
		// Pointer to array [N x T]*
		if llvmArrayType, ok := ptrType.ElemType.(*types.ArrayType); ok {
			dataPtr = ah.lcg.currentBlock.NewGetElementPtr(llvmArrayType, arrayVal, zero, zero)
		} else {
			dataPtr = arrayVal
		}
	} else {
		// Should not happen
		dataPtr = arrayVal
	}

	return ah.CreateSlice(dataPtr, arrayType.Length, elemType)
}

// IndexArray indexes into a fixed-size array
// Input: [N x T]* (pointer to array), index
// Output: T (loaded element value)
func (ah *ArrayHelper) IndexArray(arrayAlloca value.Value, index value.Value) value.Value {
	ptrType, ok := arrayAlloca.Type().(*types.PointerType)
	if !ok {
		return nil
	}

	arrayType, ok := ptrType.ElemType.(*types.ArrayType)
	if !ok {
		return nil
	}

	zero := constant.NewInt(types.I32, 0)
	elemPtr := ah.lcg.currentBlock.NewGetElementPtr(arrayType, arrayAlloca, zero, index)
	return ah.lcg.currentBlock.NewLoad(arrayType.ElemType, elemPtr)
}

// IndexSlice indexes into a slice struct
// Input: { T*, i32 } (slice struct), index
// Output: T (loaded element value)
func (ah *ArrayHelper) IndexSlice(sliceVal value.Value, index value.Value) value.Value {
	// Extract data pointer from slice struct
	dataPtr := ah.lcg.currentBlock.NewExtractValue(sliceVal, 0)

	// Get element type
	ptrType, ok := dataPtr.Type().(*types.PointerType)
	if !ok {
		return nil
	}
	elemType := ptrType.ElemType

	// Index into data pointer
	elemPtr := ah.lcg.currentBlock.NewGetElementPtr(elemType, dataPtr, index)
	return ah.lcg.currentBlock.NewLoad(elemType, elemPtr)
}

// GetArrayElementPtr gets a pointer to an array element (for assignment)
// Input: [N x T]* (pointer to array), index
// Output: T* (pointer to element)
func (ah *ArrayHelper) GetArrayElementPtr(arrayAlloca value.Value, index value.Value) value.Value {
	ptrType, ok := arrayAlloca.Type().(*types.PointerType)
	if !ok {
		return nil
	}

	arrayType, ok := ptrType.ElemType.(*types.ArrayType)
	if !ok {
		return nil
	}

	zero := constant.NewInt(types.I32, 0)
	return ah.lcg.currentBlock.NewGetElementPtr(arrayType, arrayAlloca, zero, index)
}

// GetSliceElementPtr gets a pointer to a slice element (for assignment)
// Input: { T*, i32 }* (pointer to slice struct), index
// Output: T* (pointer to element)
func (ah *ArrayHelper) GetSliceElementPtr(sliceAlloca value.Value, index value.Value) value.Value {
	// Load slice struct
	ptrType, ok := sliceAlloca.Type().(*types.PointerType)
	if !ok {
		return nil
	}

	sliceVal := ah.lcg.currentBlock.NewLoad(ptrType.ElemType, sliceAlloca)

	// Extract data pointer
	dataPtr := ah.lcg.currentBlock.NewExtractValue(sliceVal, 0)

	// Get element type
	dataPtrType, ok := dataPtr.Type().(*types.PointerType)
	if !ok {
		return nil
	}
	elemType := dataPtrType.ElemType

	// Index into data pointer
	return ah.lcg.currentBlock.NewGetElementPtr(elemType, dataPtr, index)
}

// GetLength gets the length of an array or slice
// For fixed arrays: returns constant length
// For slices: extracts length from struct
func (ah *ArrayHelper) GetLength(val value.Value, semType ortypes.Type) value.Value {
	if arrayType, ok := semType.(*ortypes.ArrayType); ok {
		if arrayType.Length >= 0 {
			// Fixed-size array - return constant
			return constant.NewInt(types.I32, arrayType.Length)
		}
		// Slice - extract length from struct
		// val should be the slice struct { T*, i32 }
		return ah.lcg.currentBlock.NewExtractValue(val, 1)
	}

	return nil
}

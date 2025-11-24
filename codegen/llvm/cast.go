package llvm

import (
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types" // Added import for types
	"github.com/llir/llvm/ir/value"
	ortypes "github.com/orktes/orlang/types"
)

func (lcg *LLVMCodeGen) castIfNeeded(val value.Value, sourceTyp, targetTyp ortypes.Type) value.Value { // Modified function signature
	if sourceTyp == nil || targetTyp == nil {
		return val
	}

	// Handle int32 -> int64 promotion
	if sourceTyp.GetName() == "int32" && targetTyp.GetName() == "int64" {
		return lcg.currentBlock.NewSExt(val, types.I64)
	}

	// Handle &int8 <-> string (both are i8*)
	if sourcePtr, ok := sourceTyp.(*ortypes.PointerType); ok {
		if targetTyp.GetName() == "string" {
			// Check if it's &int8
			if prim, ok := sourcePtr.Type.(*ortypes.PrimitiveType); ok && prim.Type == "int8" {
				return val // No cast needed, both are i8*
			}
		}
	}

	if targetPtr, ok := targetTyp.(*ortypes.PointerType); ok {
		if sourceTyp.GetName() == "string" {
			// Check if it's &int8
			if prim, ok := targetPtr.Type.(*ortypes.PrimitiveType); ok && prim.Type == "int8" {
				return val // No cast needed
			}
		}
	}

	if targetIface, ok := targetTyp.(*ortypes.InterfaceType); ok {
		if _, isIface := sourceTyp.(*ortypes.InterfaceType); !isIface {
			return lcg.createInterfaceCast(val, sourceTyp, targetIface)
		}
	}

	// Handle Fixed Array -> Slice conversion
	if sourceArray, ok := sourceTyp.(*ortypes.ArrayType); ok {
		if targetArray, ok := targetTyp.(*ortypes.ArrayType); ok {
			if sourceArray.Length >= 0 && targetArray.Length == -1 {
				// Convert fixed array (pointer) to slice struct
				// val is pointer to array [N x T]*

				// We need to construct struct { T*, i32 }

				// 1. Get pointer to first element
				// val is [N x T]*
				// GEP to [0, 0] -> T*
				zero := constant.NewInt(types.I32, 0)

				// 2. Create slice struct
				elemType := lcg.getLLVMTypeFromSemantic(targetArray.Type)
				sliceType := types.NewStruct(types.NewPointer(elemType), types.I32)

				var sliceVal value.Value = constant.NewStruct(sliceType, constant.NewNull(types.NewPointer(elemType)), constant.NewInt(types.I32, 0))

				var dataPtr value.Value

				// Check if val is already decayed (pointer to element)
				if val.Type().Equal(types.NewPointer(elemType)) {
					dataPtr = val
				} else if ptrType, ok := val.Type().(*types.PointerType); ok {
					// It's a pointer to array [N x T]*
					arrayType := ptrType.ElemType
					dataPtr = lcg.currentBlock.NewGetElementPtr(arrayType, val, zero, zero)
				} else {
					// Should not happen
					return val
				}

				sliceVal = lcg.currentBlock.NewInsertValue(sliceVal, dataPtr, 0)

				length := constant.NewInt(types.I32, sourceArray.Length)
				sliceVal = lcg.currentBlock.NewInsertValue(sliceVal, length, 1)

				return sliceVal
			}
		}
	}

	// Resolve LazyTypes
	if lazy, ok := sourceTyp.(*ortypes.LazyType); ok {
		sourceTyp = lazy.Resolver()
	}
	if lazy, ok := targetTyp.(*ortypes.LazyType); ok {
		targetTyp = lazy.Resolver()
	}

	// Check if casting to interface
	if targetIface, ok := targetTyp.(*ortypes.InterfaceType); ok {
		// If source is struct (or pointer to struct), cast to interface
		// Note: In LLVM, we usually work with pointers to structs.
		// sourceTyp from semantic analysis might be StructType.

		if _, ok := sourceTyp.(*ortypes.StructType); ok {
			return lcg.createInterfaceCast(val, sourceTyp, targetIface)
		}

		// If source is already the same interface, no cast needed (or maybe bitcast if needed?)
		// If source is another interface, we might need to adjust itable? (Not supported yet probably)
	}
	return val
}

// castValue performs explicit casting between LLVM types
// castValue performs explicit casting between LLVM types
func (lcg *LLVMCodeGen) castValue(val value.Value, targetType types.Type, sourceIsSigned, targetIsSigned bool) value.Value {
	sourceType := val.Type()

	if sourceType.Equal(targetType) {
		return val
	}

	// Helper to check if type is Int
	isInt := func(t types.Type) bool {
		_, ok := t.(*types.IntType)
		return ok
	}

	// Helper to check if type is Float
	isFloat := func(t types.Type) bool {
		_, ok := t.(*types.FloatType)
		return ok
	}

	// Int -> Float
	if isInt(sourceType) && isFloat(targetType) {
		if sourceIsSigned {
			return lcg.currentBlock.NewSIToFP(val, targetType)
		}
		return lcg.currentBlock.NewUIToFP(val, targetType)
	}

	// Float -> Int
	if isFloat(sourceType) && isInt(targetType) {
		if targetIsSigned {
			return lcg.currentBlock.NewFPToSI(val, targetType)
		}
		return lcg.currentBlock.NewFPToUI(val, targetType)
	}

	// Int -> Int (Extension/Truncation)
	if srcInt, ok := sourceType.(*types.IntType); ok {
		if dstInt, ok := targetType.(*types.IntType); ok {
			if srcInt.BitSize < dstInt.BitSize {
				if sourceIsSigned {
					return lcg.currentBlock.NewSExt(val, targetType)
				}
				return lcg.currentBlock.NewZExt(val, targetType)
			} else if srcInt.BitSize > dstInt.BitSize {
				return lcg.currentBlock.NewTrunc(val, targetType)
			}
		}
	}

	// Float -> Float (Extension/Truncation)
	if srcFloat, ok := sourceType.(*types.FloatType); ok {
		if dstFloat, ok := targetType.(*types.FloatType); ok {
			// Float is 32, Double is 64
			if srcFloat.Kind == types.FloatKindFloat && dstFloat.Kind == types.FloatKindDouble {
				return lcg.currentBlock.NewFPExt(val, targetType)
			} else if srcFloat.Kind == types.FloatKindDouble && dstFloat.Kind == types.FloatKindFloat {
				return lcg.currentBlock.NewFPTrunc(val, targetType)
			}
		}
	}

	// Fallback to bitcast
	return lcg.currentBlock.NewBitCast(val, targetType)
}

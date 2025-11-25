package llvm

import (
	"github.com/llir/llvm/ir/types" // Added import for types
	"github.com/llir/llvm/ir/value"
	ortypes "github.com/orktes/orlang/types"
)

func (lcg *LLVMCodeGen) castIfNeeded(val value.Value, sourceTyp, targetTyp ortypes.Type) value.Value {
	if sourceTyp == nil || targetTyp == nil {
		return val
	}

	// Resolve LazyTypes
	if lazy, ok := sourceTyp.(*ortypes.LazyType); ok {
		sourceTyp = lazy.Resolver()
	}
	if lazy, ok := targetTyp.(*ortypes.LazyType); ok {
		targetTyp = lazy.Resolver()
	}

	// Handle numeric promotions
	if val, ok := lcg.promoteNumeric(val, sourceTyp, targetTyp); ok {
		return val
	}

	// Handle string casts (&int8 <-> string)
	if val, ok := lcg.handleStringCast(val, sourceTyp, targetTyp); ok {
		return val
	}

	// Handle Fixed Array -> Slice conversion
	if sourceArray, ok := sourceTyp.(*ortypes.ArrayType); ok {
		if targetArray, ok := targetTyp.(*ortypes.ArrayType); ok {
			if sourceArray.Length >= 0 && targetArray.Length == -1 {
				return lcg.arrayHelper.ArrayToSlice(val, sourceArray)
			}
		}
	}

	// Handle Interface casts
	if targetIface, ok := targetTyp.(*ortypes.InterfaceType); ok {
		// If source is struct (or pointer to struct), cast to interface
		if _, ok := sourceTyp.(*ortypes.StructType); ok {
			return lcg.createInterfaceCast(val, sourceTyp, targetIface)
		}

		// Check if we're casting from another interface (not supported yet fully, but check types)
		if _, isIface := sourceTyp.(*ortypes.InterfaceType); !isIface {
			return lcg.createInterfaceCast(val, sourceTyp, targetIface)
		}
	}

	return val
}

func (lcg *LLVMCodeGen) promoteNumeric(val value.Value, sourceTyp, targetTyp ortypes.Type) (value.Value, bool) {
	// Handle int32 -> int64 promotion
	if sourceTyp.GetName() == "int32" && targetTyp.GetName() == "int64" {
		return lcg.currentBlock.NewSExt(val, types.I64), true
	}
	return nil, false
}

func (lcg *LLVMCodeGen) handleStringCast(val value.Value, sourceTyp, targetTyp ortypes.Type) (value.Value, bool) {
	// Handle &int8 -> string (both are i8*)
	if sourcePtr, ok := sourceTyp.(*ortypes.PointerType); ok {
		if targetTyp.GetName() == "string" {
			if prim, ok := sourcePtr.Type.(*ortypes.PrimitiveType); ok && prim.Type == "int8" {
				return val, true // No cast needed
			}
		}
	}

	// Handle string -> &int8
	if targetPtr, ok := targetTyp.(*ortypes.PointerType); ok {
		if sourceTyp.GetName() == "string" {
			if prim, ok := targetPtr.Type.(*ortypes.PrimitiveType); ok && prim.Type == "int8" {
				return val, true // No cast needed
			}
		}
	}

	return nil, false
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

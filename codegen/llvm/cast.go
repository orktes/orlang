package llvm

import (
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
func (lcg *LLVMCodeGen) castValue(val value.Value, targetType types.Type) value.Value {
	sourceType := val.Type()

	if sourceType.Equal(targetType) {
		return val
	}

	// Int -> Float
	if sourceType.Equal(types.I32) || sourceType.Equal(types.I64) {
		if targetType.Equal(types.Float) || targetType.Equal(types.Double) {
			return lcg.currentBlock.NewSIToFP(val, targetType)
		}
	}

	// Float -> Int
	if sourceType.Equal(types.Float) || sourceType.Equal(types.Double) {
		if targetType.Equal(types.I32) || targetType.Equal(types.I64) {
			return lcg.currentBlock.NewFPToSI(val, targetType)
		}
	}

	// Int -> Int (Extension/Truncation)
	if srcInt, ok := sourceType.(*types.IntType); ok {
		if dstInt, ok := targetType.(*types.IntType); ok {
			if srcInt.BitSize < dstInt.BitSize {
				return lcg.currentBlock.NewSExt(val, targetType)
			} else if srcInt.BitSize > dstInt.BitSize {
				return lcg.currentBlock.NewTrunc(val, targetType)
			}
		}
	}

	// Float -> Float (Extension/Truncation)
	if srcFloat, ok := sourceType.(*types.FloatType); ok {
		if dstFloat, ok := targetType.(*types.FloatType); ok {
			// Float is 32, Double is 64
			// We can check Kind or assume standard sizes
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

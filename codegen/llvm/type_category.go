package llvm

import (
	"github.com/llir/llvm/ir/types"
	ortypes "github.com/orktes/orlang/types"
)

// TypeCategory represents the value semantics of a type
type TypeCategory int

const (
	// PrimitiveType - primitives like int, float, bool (value semantics, should be loaded)
	PrimitiveType TypeCategory = iota
	// StructRefType - structs (reference semantics, should stay as pointers)
	StructRefType
	// TupleValueType - tuples (value semantics, should be loaded for unpacking)
	TupleValueType
	// ArrayDecayType - arrays (decay to pointer to first element)
	ArrayDecayType
	// InterfaceValueType - interfaces (value semantics, struct of {data*, itable*})
	InterfaceValueType
	// SliceValueType - slices (value semantics, struct of {data*, length})
	SliceValueType
)

// String returns a human-readable representation of the TypeCategory
func (tc TypeCategory) String() string {
	switch tc {
	case PrimitiveType:
		return "PrimitiveType"
	case StructRefType:
		return "StructRefType"
	case TupleValueType:
		return "TupleValueType"
	case ArrayDecayType:
		return "ArrayDecayType"
	case InterfaceValueType:
		return "InterfaceValueType"
	case SliceValueType:
		return "SliceValueType"
	default:
		return "Unknown"
	}
}

// GetTypeCategory determines the category of a type based on its semantic and LLVM representation
func GetTypeCategory(semType ortypes.Type, llvmType types.Type) TypeCategory {
	if semType == nil {
		// Fallback to LLVM type analysis
		return getTypeCategoryFromLLVM(llvmType)
	}

	// Resolve lazy types
	semType = ortypes.LazyResolve(semType)

	// Check semantic type first (most reliable)
	switch st := semType.(type) {
	case *ortypes.StructType:
		return StructRefType
	case *ortypes.TupleType:
		return TupleValueType
	case *ortypes.ArrayType:
		if st.Length == -1 {
			// Slice
			return SliceValueType
		}
		// Fixed-size array
		return ArrayDecayType
	case *ortypes.InterfaceType:
		return InterfaceValueType
	case *ortypes.PrimitiveType:
		return PrimitiveType
	case *ortypes.PointerType:
		// Pointers to primitives are still primitives in terms of loading
		return PrimitiveType
	default:
		// Unknown or complex type, fallback to LLVM analysis
		return getTypeCategoryFromLLVM(llvmType)
	}
}

// getTypeCategoryFromLLVM determines category from LLVM type alone (fallback)
func getTypeCategoryFromLLVM(llvmType types.Type) TypeCategory {
	if llvmType == nil {
		return PrimitiveType
	}

	switch t := llvmType.(type) {
	case *types.IntType, *types.FloatType:
		return PrimitiveType
	case *types.PointerType:
		// Check what it points to
		switch t.ElemType.(type) {
		case *types.StructType:
			// Could be struct, tuple, or interface - can't tell without semantic type
			// Default to primitive to be safe (will load)
			return PrimitiveType
		case *types.ArrayType:
			return ArrayDecayType
		default:
			return PrimitiveType
		}
	case *types.StructType:
		// Struct value (not pointer) - could be tuple, slice, or interface
		// Default to value type
		return TupleValueType
	case *types.ArrayType:
		return ArrayDecayType
	default:
		return PrimitiveType
	}
}

// AllocaMetadata stores type information for an alloca instruction
type AllocaMetadata struct {
	SemanticType ortypes.Type
	Category     TypeCategory
}

package llvm

import (
	"github.com/llir/llvm/ir/value"
	ortypes "github.com/orktes/orlang/types"
)

func (lcg *LLVMCodeGen) castIfNeeded(val value.Value, sourceTyp ortypes.Type, targetTyp ortypes.Type) value.Value {
	if sourceTyp == nil || targetTyp == nil {
		return val
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

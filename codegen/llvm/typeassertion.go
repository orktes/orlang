package llvm

import (
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/orktes/orlang/ast"
)

func (lcg *LLVMCodeGen) visitTypeAssertionExpression(n *ast.TypeAssertionExpression) {
	// 1. Evaluate the expression (should be an interface value)
	ast.Walk(lcg, n.Expression)
	ifaceVal := lcg.values[n.Expression]

	if ifaceVal == nil {
		return
	}

	// 2. Get the type name we're checking against
	var targetTypeName string
	if typeRef, ok := n.Type.(*ast.TypeReference); ok {
		targetTypeName = typeRef.Name.Text
	} else {
		// For now, only support simple type references
		return
	}

	// 3. Get the type ID for the target type
	targetTypeID := lcg.getTypeID(targetTypeName)

	// 4. Extract itable pointer from interface value
	// Interface is {i8* data, i8* itable}
	itablePtr := lcg.currentBlock.NewExtractValue(ifaceVal, 1)

	// 5. Cast itable pointer to the itable struct type
	// Itable is {i32 typeID, [N x i8*] methods}
	// We only need to access the first field (typeID)
	itableStructType := types.NewStruct(types.I32, types.NewArray(0, types.I8Ptr))
	itableStructPtr := lcg.currentBlock.NewBitCast(itablePtr, types.NewPointer(itableStructType))

	// 6. Load the type ID from the itable (first field, index 0)
	zero := constant.NewInt(types.I32, 0)
	typeIDPtr := lcg.currentBlock.NewGetElementPtr(itableStructType, itableStructPtr, zero, zero)
	actualTypeID := lcg.currentBlock.NewLoad(types.I32, typeIDPtr)

	// 7. Compare the actual type ID with the expected type ID
	expectedTypeID := constant.NewInt(types.I32, int64(targetTypeID))
	result := lcg.currentBlock.NewICmp(enum.IPredEQ, actualTypeID, expectedTypeID)

	// 8. Store the result (i1 boolean)
	lcg.values[n] = result
}

package llvm

import (
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/orktes/orlang/ast"
)

func (lcg *LLVMCodeGen) visitTupleExpression(n *ast.TupleExpression) {
	// Evaluate all expressions in the tuple
	var values []value.Value
	for _, expr := range n.Expressions {
		ast.Walk(lcg, expr)
		val := lcg.values[expr]
		if val == nil {
			return
		}
		values = append(values, val)
	}

	// Get the tuple type from semantic analysis
	nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n]
	if nodeInfo == nil || nodeInfo.Type == nil {
		return
	}

	tupleType := lcg.getLLVMTypeFromSemantic(nodeInfo.Type)
	structType, ok := tupleType.(*types.StructType)
	if !ok {
		return
	}

	// Allocate space for the tuple on the stack
	alloca := lcg.currentBlock.NewAlloca(structType)

	// Store each element into the struct
	for i, val := range values {
		// Get pointer to field
		zero := constant.NewInt(types.I32, 0)
		idx := constant.NewInt(types.I32, int64(i))
		fieldPtr := lcg.currentBlock.NewGetElementPtr(structType, alloca, zero, idx)

		// Store value
		lcg.currentBlock.NewStore(val, fieldPtr)
	}

	// Load the complete struct
	tupleVal := lcg.currentBlock.NewLoad(structType, alloca)
	lcg.values[n] = tupleVal
}

func (lcg *LLVMCodeGen) visitTupleDeclaration(n *ast.TupleDeclaration) {
	// Evaluate the tuple expression
	ast.Walk(lcg, n.DefaultValue)
	tupleVal := lcg.values[n.DefaultValue]

	if tupleVal == nil {
		return
	}

	// Get the tuple type
	var tupleType *types.StructType
	if structType, ok := tupleVal.Type().(*types.StructType); ok {
		tupleType = structType
	} else {
		return
	}

	// Extract each element and create variables
	for i, pattern := range n.Pattern.Patterns {
		if i >= len(tupleType.Fields) {
			break
		}

		// Pattern should be an Identifier
		ident, ok := pattern.(*ast.Identifier)
		if !ok {
			continue
		}

		// Extract the element
		elemVal := lcg.currentBlock.NewExtractValue(tupleVal, uint64(i))

		// Create alloca for the variable
		alloca := lcg.currentBlock.NewAlloca(tupleType.Fields[i])
		lcg.currentBlock.NewStore(elemVal, alloca)

		// Store the alloca address
		lcg.values[ident] = alloca
	}
}

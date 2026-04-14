package llvm

import (
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/orktes/orlang/ast"
)

func (lcg *LLVMCodeGen) visitComparisonExpression(n *ast.ComparisonExpression) {
	// Short-circuit evaluation for && and ||
	if n.Operator.Text == "&&" {
		lcg.visitLogicalAnd(n)
		return
	}
	if n.Operator.Text == "||" {
		lcg.visitLogicalOr(n)
		return
	}

	ast.Walk(lcg, n.Left)
	ast.Walk(lcg, n.Right)

	leftVal := lcg.values[n.Left]
	rightVal := lcg.values[n.Right]

	if leftVal == nil || rightVal == nil {
		return
	}

	var val value.Value

	// Check if both operands are strings (i8*) — use strcmp for comparison
	isString := false
	if ptrL, ok := leftVal.Type().(*types.PointerType); ok {
		if ptrR, ok := rightVal.Type().(*types.PointerType); ok {
			if ptrL.ElemType.Equal(types.I8) && ptrR.ElemType.Equal(types.I8) {
				isString = true
			}
		}
	}

	if isString {
		// Use strcmp for string comparison
		var strcmpFn *ir.Func
		if fn, ok := lcg.functions["strcmp"]; ok {
			strcmpFn = fn
		} else {
			strcmpFn = lcg.module.NewFunc("strcmp", types.I32,
				ir.NewParam("s1", types.I8Ptr),
				ir.NewParam("s2", types.I8Ptr))
			lcg.functions["strcmp"] = strcmpFn
		}
		cmpResult := lcg.currentBlock.NewCall(strcmpFn, leftVal, rightVal)

		switch n.Operator.Text {
		case "==":
			val = lcg.currentBlock.NewICmp(enum.IPredEQ, cmpResult, constant.NewInt(types.I32, 0))
		case "!=":
			val = lcg.currentBlock.NewICmp(enum.IPredNE, cmpResult, constant.NewInt(types.I32, 0))
		case "<":
			val = lcg.currentBlock.NewICmp(enum.IPredSLT, cmpResult, constant.NewInt(types.I32, 0))
		case "<=":
			val = lcg.currentBlock.NewICmp(enum.IPredSLE, cmpResult, constant.NewInt(types.I32, 0))
		case ">":
			val = lcg.currentBlock.NewICmp(enum.IPredSGT, cmpResult, constant.NewInt(types.I32, 0))
		case ">=":
			val = lcg.currentBlock.NewICmp(enum.IPredSGE, cmpResult, constant.NewInt(types.I32, 0))
		}
	} else {
		switch n.Operator.Text {
		case "==":
			val = lcg.currentBlock.NewICmp(enum.IPredEQ, leftVal, rightVal)
		case "!=":
			val = lcg.currentBlock.NewICmp(enum.IPredNE, leftVal, rightVal)
		case "<":
			val = lcg.currentBlock.NewICmp(enum.IPredSLT, leftVal, rightVal)
		case "<=":
			val = lcg.currentBlock.NewICmp(enum.IPredSLE, leftVal, rightVal)
		case ">":
			val = lcg.currentBlock.NewICmp(enum.IPredSGT, leftVal, rightVal)
		case ">=":
			val = lcg.currentBlock.NewICmp(enum.IPredSGE, leftVal, rightVal)
		}
	}

	lcg.values[n] = val
}

// visitLogicalAnd implements short-circuit && evaluation
// if left is false, result is false (skip right)
func (lcg *LLVMCodeGen) visitLogicalAnd(n *ast.ComparisonExpression) {
	// Evaluate left side
	ast.Walk(lcg, n.Left)
	leftVal := lcg.values[n.Left]
	if leftVal == nil {
		return
	}

	// Ensure left is i1
	leftBool := lcg.ensureBool(leftVal)

	// Create blocks
	rightBlock := lcg.currentFunc.NewBlock("")
	mergeBlock := lcg.currentFunc.NewBlock("")

	// Record the block where left was evaluated
	leftBlock := lcg.currentBlock

	// Branch: if left is true, evaluate right; else short-circuit to false
	lcg.currentBlock.NewCondBr(leftBool, rightBlock, mergeBlock)

	// Right block: evaluate right side
	lcg.currentBlock = rightBlock
	ast.Walk(lcg, n.Right)
	rightVal := lcg.values[n.Right]
	if rightVal == nil {
		rightVal = constant.False
	}
	rightBool := lcg.ensureBool(rightVal)
	rightEndBlock := lcg.currentBlock
	lcg.currentBlock.NewBr(mergeBlock)

	// Merge block: phi node selects the result
	lcg.currentBlock = mergeBlock
	phi := mergeBlock.NewPhi(
		&ir.Incoming{X: constant.False, Pred: leftBlock},
		&ir.Incoming{X: rightBool, Pred: rightEndBlock},
	)

	lcg.values[n] = phi
}

// visitLogicalOr implements short-circuit || evaluation
// if left is true, result is true (skip right)
func (lcg *LLVMCodeGen) visitLogicalOr(n *ast.ComparisonExpression) {
	// Evaluate left side
	ast.Walk(lcg, n.Left)
	leftVal := lcg.values[n.Left]
	if leftVal == nil {
		return
	}

	// Ensure left is i1
	leftBool := lcg.ensureBool(leftVal)

	// Create blocks
	rightBlock := lcg.currentFunc.NewBlock("")
	mergeBlock := lcg.currentFunc.NewBlock("")

	// Record the block where left was evaluated
	leftBlock := lcg.currentBlock

	// Branch: if left is true, short-circuit to true; else evaluate right
	lcg.currentBlock.NewCondBr(leftBool, mergeBlock, rightBlock)

	// Right block: evaluate right side
	lcg.currentBlock = rightBlock
	ast.Walk(lcg, n.Right)
	rightVal := lcg.values[n.Right]
	if rightVal == nil {
		rightVal = constant.False
	}
	rightBool := lcg.ensureBool(rightVal)
	rightEndBlock := lcg.currentBlock
	lcg.currentBlock.NewBr(mergeBlock)

	// Merge block: phi node selects the result
	lcg.currentBlock = mergeBlock
	phi := mergeBlock.NewPhi(
		&ir.Incoming{X: constant.True, Pred: leftBlock},
		&ir.Incoming{X: rightBool, Pred: rightEndBlock},
	)

	lcg.values[n] = phi
}

// ensureBool converts a value to i1 if it isn't already
func (lcg *LLVMCodeGen) ensureBool(v value.Value) value.Value {
	if v.Type().Equal(types.I1) {
		return v
	}
	// Compare != 0 to convert integer to bool
	if intType, ok := v.Type().(*types.IntType); ok {
		return lcg.currentBlock.NewICmp(enum.IPredNE, v, constant.NewInt(intType, 0))
	}
	return v
}

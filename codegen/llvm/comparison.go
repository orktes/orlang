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

	preds, knownOp := comparisonPredicates[n.Operator.Text]
	if !knownOp {
		lcg.errorf(n, "unsupported comparison operator %q", n.Operator.Text)
		return
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
		val = lcg.currentBlock.NewICmp(preds.signed, cmpResult, constant.NewInt(types.I32, 0))
	} else if isFloatLLVMType(leftVal.Type()) || isFloatLLVMType(rightVal.Type()) {
		leftVal, rightVal = lcg.unifyNumericOperands(leftVal, rightVal, n.Left, n.Right)
		val = lcg.currentBlock.NewFCmp(preds.float, leftVal, rightVal)
	} else if lcg.operandsUnsigned(n.Left, n.Right) {
		leftVal, rightVal = lcg.unifyNumericOperands(leftVal, rightVal, n.Left, n.Right)
		val = lcg.currentBlock.NewICmp(preds.unsigned, leftVal, rightVal)
	} else {
		leftVal, rightVal = lcg.unifyNumericOperands(leftVal, rightVal, n.Left, n.Right)
		val = lcg.currentBlock.NewICmp(preds.signed, leftVal, rightVal)
	}

	lcg.values[n] = val
}

// unifyNumericOperands makes both operands of an arithmetic or comparison
// operation the same LLVM type: ints are converted to the float side's type
// when mixed, the narrower float is extended, and mixed-width ints widen to
// the wider operand (constants are simply retyped).
func (lcg *LLVMCodeGen) unifyNumericOperands(left, right value.Value, leftNode, rightNode ast.Node) (value.Value, value.Value) {
	lf := isFloatLLVMType(left.Type())
	rf := isFloatLLVMType(right.Type())

	switch {
	case lf && !rf:
		right = lcg.numericConvert(right, left.Type(), rightNode)
	case rf && !lf:
		left = lcg.numericConvert(left, right.Type(), leftNode)
	case lf && rf && !left.Type().Equal(right.Type()):
		if left.Type().Equal(types.Double) {
			right = lcg.currentBlock.NewFPExt(right, types.Double)
		} else {
			left = lcg.currentBlock.NewFPExt(left, types.Double)
		}
	case !lf && !rf:
		lInt, lok := left.Type().(*types.IntType)
		rInt, rok := right.Type().(*types.IntType)
		if lok && rok && lInt.BitSize != rInt.BitSize {
			if lInt.BitSize < rInt.BitSize {
				left = lcg.numericConvert(left, right.Type(), leftNode)
			} else {
				right = lcg.numericConvert(right, left.Type(), rightNode)
			}
		}
	}
	return left, right
}

// numericConvert converts a value to the target LLVM numeric type, retyping
// integer/float constants directly and using signedness-aware casts for
// non-constant values.
func (lcg *LLVMCodeGen) numericConvert(val value.Value, target types.Type, node ast.Node) value.Value {
	if val.Type().Equal(target) {
		return val
	}
	if c, ok := val.(*constant.Int); ok {
		if intType, ok := target.(*types.IntType); ok {
			return constant.NewInt(intType, c.X.Int64())
		}
		if floatType, ok := target.(*types.FloatType); ok {
			return constant.NewFloat(floatType, float64(c.X.Int64()))
		}
	}
	if c, ok := val.(*constant.Float); ok {
		if floatType, ok := target.(*types.FloatType); ok {
			f, _ := c.X.Float64()
			return constant.NewFloat(floatType, f)
		}
	}
	signed := !isUnsignedType(lcg.semanticType(node))
	return lcg.castValue(val, target, signed, signed)
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

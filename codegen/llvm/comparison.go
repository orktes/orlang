package llvm

import (
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/value"
	"github.com/orktes/orlang/ast"
)

func (lcg *LLVMCodeGen) visitComparisonExpression(n *ast.ComparisonExpression) {
	ast.Walk(lcg, n.Left)
	ast.Walk(lcg, n.Right)

	leftVal := lcg.values[n.Left]
	rightVal := lcg.values[n.Right]

	if leftVal == nil || rightVal == nil {
		return
	}

	var val value.Value

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

	lcg.values[n] = val
}

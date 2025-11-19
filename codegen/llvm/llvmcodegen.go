package llvm

import (
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/orktes/orlang/analyser"
	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

type LLVMCodeGen struct {
	analyserInfo *analyser.Info
	module       *ir.Module
	currentBlock *ir.Block
	currentFunc  *ir.Func
	currentFile  *ast.File
	values       map[ast.Node]value.Value
	functions    map[string]*ir.Func
}

func New(info *analyser.Info) *LLVMCodeGen {
	return &LLVMCodeGen{
		analyserInfo: info,
		module:       ir.NewModule(),
		values:       make(map[ast.Node]value.Value),
		functions:    make(map[string]*ir.Func),
	}
}

func (lcg *LLVMCodeGen) Generate(file *ast.File) string {
	lcg.currentFile = file
	ast.Walk(lcg, file)
	return lcg.module.String()
}

func (lcg *LLVMCodeGen) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.File:
		return lcg
	case *ast.FunctionDeclaration:
		lcg.visitFunctionDeclaration(n)
		return nil
	case *ast.Block:
		for _, stmt := range n.Body {
			ast.Walk(lcg, stmt)
		}
		return nil
	case *ast.ReturnStatement:
		lcg.visitReturnStatement(n)
		return nil
	case *ast.ValueExpression:
		lcg.visitValueExpression(n)
		return nil
	case *ast.BinaryExpression:
		lcg.visitBinaryExpression(n)
		return nil
	case *ast.ParenExpression:
		lcg.visitParenExpression(n)
		return nil
	case *ast.FunctionCall:
		lcg.visitFunctionCall(n)
		return nil
	case *ast.Identifier:
		lcg.visitIdentifier(n)
		return nil
	case *ast.VariableDeclaration:
		lcg.visitVariableDeclaration(n)
		return nil
	case *ast.Assigment:
		lcg.visitAssigment(n)
		return nil
	}
	return nil
}

func (lcg *LLVMCodeGen) visitVariableDeclaration(n *ast.VariableDeclaration) {
	// Allocate memory on stack
	// TODO: Handle types
	alloca := lcg.currentBlock.NewAlloca(types.I32)
	lcg.values[n.Name] = alloca

	if n.DefaultValue != nil {
		ast.Walk(lcg, n.DefaultValue)
		val := lcg.values[n.DefaultValue]
		lcg.currentBlock.NewStore(val, alloca)
	}
}

func (lcg *LLVMCodeGen) visitAssigment(n *ast.Assigment) {
	ast.Walk(lcg, n.Right)
	val := lcg.values[n.Right]

	// Resolve left side to address
	// For now assuming left is identifier
	if ident, ok := n.Left.(*ast.Identifier); ok {
		nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[ident]
		if nodeInfo != nil {
			details := nodeInfo.Scope.GetDetails(ident.Text, true)
			if details != nil {
				if alloca, ok := lcg.values[details.DefineIdentifier]; ok {
					lcg.currentBlock.NewStore(val, alloca)
				}
			}
		}
	}
}

func (lcg *LLVMCodeGen) visitParenExpression(n *ast.ParenExpression) {
	ast.Walk(lcg, n.Expression)
	lcg.values[n] = lcg.values[n.Expression]
}

func (lcg *LLVMCodeGen) visitBinaryExpression(n *ast.BinaryExpression) {
	ast.Walk(lcg, n.Left)
	ast.Walk(lcg, n.Right)

	leftVal := lcg.values[n.Left]
	rightVal := lcg.values[n.Right]

	var val value.Value

	switch n.Operator.Text {
	case "+":
		val = lcg.currentBlock.NewAdd(leftVal, rightVal)
	case "-":
		val = lcg.currentBlock.NewSub(leftVal, rightVal)
	case "*":
		val = lcg.currentBlock.NewMul(leftVal, rightVal)
	case "/":
		val = lcg.currentBlock.NewSDiv(leftVal, rightVal)
	}

	lcg.values[n] = val
}

func (lcg *LLVMCodeGen) visitFunctionDeclaration(n *ast.FunctionDeclaration) {
	name := "main"
	if n.Signature.Identifier != nil {
		name = n.Signature.Identifier.Text
	}

	var params []*ir.Param
	for _, arg := range n.Signature.Arguments {
		// TODO: Map types correctly
		var paramType types.Type = types.I32
		if arg.Type != nil {
			// Very basic type mapping for now
			if typeRef, ok := arg.Type.(*ast.TypeReference); ok {
				if typeRef.Name.Text == "string" {
					paramType = types.I8Ptr
				}
			}
		}

		param := ir.NewParam(arg.Name.Text, paramType)
		params = append(params, param)
		lcg.values[arg.Name] = param
	}

	fn := lcg.module.NewFunc(name, types.I32, params...)
	lcg.functions[name] = fn

	if n.Signature.Extern {
		return
	}

	block := fn.NewBlock("")

	lcg.currentFunc = fn
	lcg.currentBlock = block

	// Alloca parameters so they are mutable/addressable
	for i, param := range params {
		alloca := block.NewAlloca(types.I32)
		block.NewStore(param, alloca)
		// Update mapping to point to alloca
		argName := n.Signature.Arguments[i].Name
		lcg.values[argName] = alloca
	}

	ast.Walk(lcg, n.Block)

	// Add implicit return 0 if missing (for main)
	if name == "main" && (len(n.Block.Body) == 0 || !lcg.isTerminator(lcg.currentBlock.Term)) {
		lcg.currentBlock.NewRet(constant.NewInt(types.I32, 0))
	}
}

func (lcg *LLVMCodeGen) visitFunctionCall(n *ast.FunctionCall) {
	var name string
	if ident, ok := n.Callee.(*ast.Identifier); ok {
		name = ident.Text
	} else {
		// TODO: Handle other callees
		return
	}

	fn, ok := lcg.functions[name]
	if !ok {
		// TODO: Handle undefined function
		return
	}

	var args []value.Value
	for _, arg := range n.Arguments {
		ast.Walk(lcg, arg.Expression)
		args = append(args, lcg.values[arg.Expression])
	}

	val := lcg.currentBlock.NewCall(fn, args...)
	lcg.values[n] = val
}

func (lcg *LLVMCodeGen) visitIdentifier(n *ast.Identifier) {
	nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n]
	if nodeInfo == nil {
		return
	}

	details := nodeInfo.Scope.GetDetails(n.Text, true)
	if details == nil {
		return
	}

	if val, ok := lcg.values[details.DefineIdentifier]; ok {
		// If it's an alloca (pointer), load it
		if _, isPtr := val.Type().(*types.PointerType); isPtr {
			// Check if it's a function parameter (which is also a value but not a pointer to stack usually in this impl)
			// Actually parameters in LLVM IR are values, but if we want mutable variables we usually alloca them.
			// For now, if it's an alloca (instruction), load it.
			if _, isInst := val.(ir.Instruction); isInst {
				load := lcg.currentBlock.NewLoad(types.I32, val)
				lcg.values[n] = load
				return
			}
		}
		lcg.values[n] = val
	}
}

func (lcg *LLVMCodeGen) visitReturnStatement(n *ast.ReturnStatement) {
	if n.Expression != nil {
		ast.Walk(lcg, n.Expression)
		val := lcg.values[n.Expression]
		lcg.currentBlock.NewRet(val)
	} else {
		lcg.currentBlock.NewRet(nil)
	}
}

func (lcg *LLVMCodeGen) visitValueExpression(n *ast.ValueExpression) {
	// TODO: Handle other types
	// Assuming integer for now
	var val value.Value

	if n.Token.Type == scanner.TokenTypeString {
		strVal := n.Token.Value.(string)
		val = lcg.addStringConstant(strVal)
	} else if n.Token.Value != nil {
		if i, ok := n.Token.Value.(int64); ok {
			val = constant.NewInt(types.I32, i)
		}
	}

	lcg.values[n] = val
}

func (lcg *LLVMCodeGen) addStringConstant(str string) value.Value {
	// Add null terminator
	str += "\x00"
	c := constant.NewCharArrayFromString(str)
	g := lcg.module.NewGlobalDef("", c)
	g.Immutable = true

	// Get pointer to first element
	zero := constant.NewInt(types.I32, 0)
	return constant.NewGetElementPtr(c.Type(), g, zero, zero)
}

func (lcg *LLVMCodeGen) isTerminator(term ir.Terminator) bool {
	return term != nil
}

func (lcg *LLVMCodeGen) Leave(node ast.Node) {}

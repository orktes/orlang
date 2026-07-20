package llvm

import (
	"sort"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/orktes/orlang/ast"
	ortypes "github.com/orktes/orlang/types"
)

// shortTypeName is the canonical runtime name of a type: the declared name
// for named structs and interfaces, the descriptive GetName for everything
// else. It doubles as the type-ID key so `is` assertions, itables and
// typename all agree.
func shortTypeName(typ ortypes.Type) string {
	typ = ortypes.LazyResolve(typ)
	if typ == nil {
		return "unknown"
	}
	switch t := typ.(type) {
	case *ortypes.StructType:
		if t.Name != "" {
			return t.Name
		}
	case *ortypes.InterfaceType:
		if t.Name != "" {
			return t.Name
		}
	}
	return typ.GetName()
}

// visitTypeofBuiltin lowers typeof(expr) to the static type name of the
// expression as a string constant. The argument is not evaluated.
func (lcg *LLVMCodeGen) visitTypeofBuiltin(n *ast.FunctionCall) {
	if len(n.Arguments) != 1 {
		return
	}
	lcg.values[n] = lcg.addStringConstant(shortTypeName(lcg.semanticType(n.Arguments[0].Expression)))
}

// visitTypenameBuiltin lowers typename(expr). For interface-typed values
// (including `anything`) the dynamic type name is looked up at runtime
// through the value's itable; for every other type it is the same as
// typeof.
func (lcg *LLVMCodeGen) visitTypenameBuiltin(n *ast.FunctionCall) {
	if len(n.Arguments) != 1 {
		return
	}
	arg := n.Arguments[0].Expression
	typ := lcg.semanticType(arg)

	if _, isIface := typ.(*ortypes.InterfaceType); !isIface {
		// Static types have static names
		lcg.visitTypeofBuiltin(n)
		return
	}

	ast.Walk(lcg, arg)
	val := lcg.values[arg]
	if val == nil {
		lcg.errorf(arg, "cannot evaluate typename argument")
		return
	}
	// Interface values are { i8* data, i8* itable }; a pointer means the
	// variable's storage — load it.
	if ptr, ok := val.Type().(*types.PointerType); ok && ptr.ElemType.Equal(lcg.getInterfaceType()) {
		val = lcg.currentBlock.NewLoad(lcg.getInterfaceType(), val)
	}
	if !val.Type().Equal(lcg.getInterfaceType()) {
		lcg.visitTypeofBuiltin(n)
		return
	}

	itable := lcg.currentBlock.NewExtractValue(val, 1)
	lcg.values[n] = lcg.currentBlock.NewCall(lcg.getOrDeclareTypenameFn(), itable)
}

// getOrDeclareTypenameFn returns __orlang_typename(i8* itable) => i8*,
// whose body (a typeID -> name switch) is generated at the end of module
// generation once every type ID is known.
func (lcg *LLVMCodeGen) getOrDeclareTypenameFn() *ir.Func {
	if lcg.typenameFn != nil {
		return lcg.typenameFn
	}
	lcg.typenameFn = lcg.module.NewFunc("__orlang_typename", types.I8Ptr,
		ir.NewParam("itable", types.I8Ptr))
	return lcg.typenameFn
}

// finalizeTypenameFn emits the body of __orlang_typename once the walk has
// assigned all type IDs (IDs are created when itables are, so the table is
// complete for every value that can exist inside an interface).
func (lcg *LLVMCodeGen) finalizeTypenameFn() {
	if lcg.typenameFn == nil {
		return
	}
	fn := lcg.typenameFn
	entry := fn.NewBlock("")

	unknown := lcg.addStringConstant("unknown")

	// null itable: zero-value interface
	loadBlock := fn.NewBlock("")
	nullBlock := fn.NewBlock("")
	isNull := entry.NewICmp(enum.IPredEQ, fn.Params[0], constant.NewNull(types.I8Ptr))
	entry.NewCondBr(isNull, nullBlock, loadBlock)
	nullBlock.NewRet(unknown)

	// itable layout: { i32 typeID, ... } — read the leading i32
	idPtr := loadBlock.NewBitCast(fn.Params[0], types.NewPointer(types.I32))
	id := loadBlock.NewLoad(types.I32, idPtr)

	// Deterministic dispatch over the registered type IDs
	names := make([]string, 0, len(lcg.typeIDs))
	for name := range lcg.typeIDs {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return lcg.typeIDs[names[i]] < lcg.typeIDs[names[j]]
	})

	current := loadBlock
	for _, name := range names {
		match := fn.NewBlock("")
		next := fn.NewBlock("")
		cond := current.NewICmp(enum.IPredEQ, id,
			constant.NewInt(types.I32, int64(lcg.typeIDs[name])))
		current.NewCondBr(cond, match, next)
		match.NewRet(lcg.addStringConstant(name))
		current = next
	}
	current.NewRet(unknown)
}

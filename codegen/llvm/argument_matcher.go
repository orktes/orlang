package llvm

import (
	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/orktes/orlang/ast"
	ortypes "github.com/orktes/orlang/types"
)

// ArgumentMatcher handles matching AST arguments to LLVM function parameters
type ArgumentMatcher struct {
	lcg       *LLVMCodeGen
	signature *ortypes.SignatureType
	fn        *ir.Func
	preArgs   []value.Value
}

// NewArgumentMatcher creates a new argument matcher
func NewArgumentMatcher(lcg *LLVMCodeGen, signature *ortypes.SignatureType, fn *ir.Func, preArgs []value.Value) *ArgumentMatcher {
	return &ArgumentMatcher{
		lcg:       lcg,
		signature: signature,
		fn:        fn,
		preArgs:   preArgs,
	}
}

// Match processes the AST arguments and returns a list of LLVM values ready for the function call
func (am *ArgumentMatcher) Match(astArgs []*ast.CallArgument) ([]value.Value, error) {
	preArgsCount := len(am.preArgs)
	numArgs := len(astArgs) + preArgsCount

	// If function has more params than args (e.g. default values or named args filling gaps),
	// we need to ensure we allocate enough space.
	// However, for now we assume the parser/analyzer ensures validity or we fill what we have.
	// But actually, we need to match the function parameters count if it's not variadic.
	if len(am.fn.Params) > numArgs {
		numArgs = len(am.fn.Params)
	}

	// Initialize ordered args with pre-filled args (e.g. 'this')
	orderedArgs := make([]value.Value, numArgs)
	for i, v := range am.preArgs {
		orderedArgs[i] = v
	}

	// Track which indices are filled (for variadic handling if needed, though mostly for debugging/validation)
	// filledIndices := make(map[int]bool)

	for i, arg := range astArgs {
		// Evaluate argument expression
		ast.Walk(am.lcg, arg.Expression)
		val := am.lcg.values[arg.Expression]

		targetIndex := i

		// Handle named arguments
		if arg.Name != nil && am.signature != nil {
			for idx, name := range am.signature.ArgumentNames {
				if name == arg.Name.Text {
					targetIndex = idx
					break
				}
			}
		}

		// Shift index by preArgsCount (to account for 'this')
		realIndex := targetIndex + preArgsCount

		// Ensure we don't go out of bounds if something is wrong
		if realIndex >= len(orderedArgs) {
			// If variadic, append
			if am.fn.Sig.Variadic {
				orderedArgs = append(orderedArgs, nil)
				// realIndex is now valid for the appended slice
			} else {
				// Should not happen if analyzer passed
				continue
			}
		}

		// Handle casting and promotions
		if realIndex < len(am.fn.Params) {
			// Regular parameter
			val = am.handleCast(val, arg.Expression, targetIndex)
		} else {
			// Variadic argument
			val = am.handleVariadicPromotion(val)
		}

		if realIndex < len(orderedArgs) {
			orderedArgs[realIndex] = val
		} else {
			orderedArgs = append(orderedArgs, val)
		}
	}

	// Filter out nils if any (e.g. optional args not implemented yet, or just safety)
	// Actually, we should probably keep them if the function expects them, but LLVM call needs valid values.
	// For variadic calls, we might have expanded orderedArgs.

	// Compact the arguments for the call
	finalArgs := make([]value.Value, 0, len(orderedArgs))
	for _, v := range orderedArgs {
		if v != nil {
			finalArgs = append(finalArgs, v)
		}
	}

	return finalArgs, nil
}

func (am *ArgumentMatcher) handleCast(val value.Value, expr ast.Expression, targetIndex int) value.Value {
	// Resolve source type
	var sourceTyp ortypes.Type
	nodeInfoArg := am.lcg.analyserInfo.FileInfo[am.lcg.currentFile].NodeInfo[expr]
	if nodeInfoArg != nil {
		sourceTyp = nodeInfoArg.Type
	}

	// Resolve target type
	var targetTyp ortypes.Type
	if am.signature != nil && targetIndex < len(am.signature.ArgumentTypes) {
		targetTyp = am.signature.ArgumentTypes[targetIndex]
	}

	if sourceTyp != nil && targetTyp != nil {
		// Special handling for struct-to-interface casts
		// We need to pass the address of the struct, not the loaded value
		if _, isTargetIface := targetTyp.(*ortypes.InterfaceType); isTargetIface {
			if _, isSourceStruct := sourceTyp.(*ortypes.StructType); isSourceStruct {
				// Get the address of the struct instead of the loaded value
				addr := am.lcg.getAddress(expr)
				if addr != nil {
					val = addr
				}
			}
		}
		return am.lcg.castIfNeeded(val, sourceTyp, targetTyp)
	}

	// Fallback to simple bitcast if types don't match
	// We need the param type from the function
	// We need to map targetIndex to real param index (accounting for preArgs)
	realIndex := targetIndex + len(am.preArgs)
	if realIndex < len(am.fn.Params) {
		param := am.fn.Params[realIndex]
		if val != nil && val.Type() != param.Type() {
			return am.lcg.currentBlock.NewBitCast(val, param.Type())
		}
	}

	return val
}

func (am *ArgumentMatcher) handleVariadicPromotion(val value.Value) value.Value {
	if val == nil {
		return nil
	}
	// Promote float to double for C compatibility (printf etc)
	if val.Type().Equal(types.Float) {
		return am.lcg.currentBlock.NewFPExt(val, types.Double)
	}
	// Promote i1, i8, i16 to i32
	if intType, ok := val.Type().(*types.IntType); ok && intType.BitSize < 32 {
		return am.lcg.currentBlock.NewZExt(val, types.I32)
	}
	return val
}

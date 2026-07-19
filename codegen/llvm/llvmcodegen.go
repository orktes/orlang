package llvm

import (
	"fmt"
	"math"
	"runtime"
	"strings"

	"github.com/llir/llvm/ir"
	"github.com/llir/llvm/ir/constant"
	"github.com/llir/llvm/ir/enum"
	"github.com/llir/llvm/ir/types"
	"github.com/llir/llvm/ir/value"
	"github.com/orktes/orlang/analyser"
	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
	ortypes "github.com/orktes/orlang/types"
)

type LLVMCodeGen struct {
	analyserInfo      *analyser.Info
	module            *ir.Module
	currentBlock      *ir.Block
	currentFunc       *ir.Func
	currentFile       *ast.File
	currentStruct     *types.StructType
	currentStructName string
	moduleName        string // Name of the current module being compiled (e.g., "lib", "main")
	values            map[ast.Node]value.Value
	functions         map[string]*ir.Func
	structs           map[string]types.Type
	structDefinitions map[string]*types.StructType
	structFields      map[string]map[string]int
	typeIDs           map[string]int32                // Type name -> Type ID for runtime type checking
	nextTypeID        int32                           // Next available type ID
	stringCount       int                             // Counter for unique string constants
	allocaMetadata    map[value.Value]*AllocaMetadata // Type metadata for alloca instructions
	arrayHelper       *ArrayHelper                    // Helper for array/slice operations
	loopExitBlocks    []*ir.Block                     // Stack of break targets (afterBlock)
	loopPostBlocks    []*ir.Block                     // Stack of continue targets (postBlock)
	deferredCalls     []ast.Expression                // Stack of deferred call expressions for current function
	enumValues        map[string]map[string]int32     // Enum name -> value name -> int32 constant
	anonFnCounter     int                             // Counter for unique anonymous function names
	closureWrappers   map[string]*ir.Func             // function name -> closure wrapper (adds env param)
	errors            []error                         // Codegen errors collected during Generate
}

func New(info *analyser.Info) *LLVMCodeGen {
	lcg := &LLVMCodeGen{
		analyserInfo:      info,
		module:            ir.NewModule(),
		values:            make(map[ast.Node]value.Value),
		functions:         make(map[string]*ir.Func),
		structs:           make(map[string]types.Type),
		structDefinitions: make(map[string]*types.StructType),
		structFields:      make(map[string]map[string]int),
		typeIDs:           make(map[string]int32),
		nextTypeID:        1, // Start from 1, reserve 0 for unknown/nil
		allocaMetadata:    make(map[value.Value]*AllocaMetadata),
		enumValues:        make(map[string]map[string]int32),
		closureWrappers:   make(map[string]*ir.Func),
	}
	lcg.initializeModule() // Call the new initialization function
	// Initialize helpers that need reference to lcg
	lcg.arrayHelper = NewArrayHelper(lcg)
	return lcg
}

func (lcg *LLVMCodeGen) initializeModule() {
	// Set target triple and data layout for proper code generation
	// Use version-specific triple to avoid clang override warnings
	var targetTriple string
	var dataLayout string

	if runtime.GOOS == "darwin" {
		if runtime.GOARCH == "arm64" {
			targetTriple = "arm64-apple-macosx14.0.0"
			dataLayout = "e-m:o-i64:64-i128:128-n32:64-S128"
		} else {
			targetTriple = "x86_64-apple-macosx10.15.0"
			dataLayout = "e-m:o-i64:64-f80:128-n8:16:32:64-S128"
		}
	} else {
		targetTriple = runtime.GOARCH + "-unknown-linux-gnu"
		if runtime.GOARCH == "arm64" {
			dataLayout = "e-m:e-i64:64-i128:128-n32:64-S128"
		} else {
			dataLayout = "e-m:e-i64:64-f80:128-n8:16:32:64-S128"
		}
	}

	lcg.module.TargetTriple = targetTriple
	lcg.module.DataLayout = dataLayout
}

func (lcg *LLVMCodeGen) SetModuleName(name string) {
	lcg.moduleName = name
}

func (lcg *LLVMCodeGen) Generate(file *ast.File) (code string) {
	// Codegen bugs and unexpected inputs surface as panics deep in the
	// visitor; report them as errors instead of crashing the compiler.
	defer func() {
		if r := recover(); r != nil {
			lcg.errors = append(lcg.errors, fmt.Errorf("internal codegen error: %v", r))
		}
	}()
	lcg.currentFile = file
	ast.Walk(lcg, file)
	return lcg.module.String()
}

func getKeys(m map[string]*ir.Func) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// getSizeOf returns the size in bytes of an LLVM type, including struct
// field alignment padding (matching the platform C ABI layout clang uses).
func (lcg *LLVMCodeGen) getSizeOf(t types.Type) int64 {
	size, _ := lcg.sizeAndAlignOf(t)
	return size
}

// sizeAndAlignOf computes the size and alignment of an LLVM type following
// standard C struct layout rules: each field is aligned to its natural
// alignment, and the struct is padded to a multiple of its widest field.
func (lcg *LLVMCodeGen) sizeAndAlignOf(t types.Type) (size int64, align int64) {
	roundUp := func(v, a int64) int64 {
		return (v + a - 1) / a * a
	}

	switch typ := t.(type) {
	case *types.IntType:
		s := int64(typ.BitSize+7) / 8
		if s == 0 {
			s = 1
		}
		return s, s
	case *types.FloatType:
		if typ.Kind == types.FloatKindDouble {
			return 8, 8
		}
		return 4, 4
	case *types.PointerType:
		return 8, 8 // Assuming 64-bit pointers
	case *types.StructType:
		offset := int64(0)
		maxAlign := int64(1)
		for _, field := range typ.Fields {
			fsize, falign := lcg.sizeAndAlignOf(field)
			offset = roundUp(offset, falign) + fsize
			if falign > maxAlign {
				maxAlign = falign
			}
		}
		return roundUp(offset, maxAlign), maxAlign
	case *types.ArrayType:
		esize, ealign := lcg.sizeAndAlignOf(typ.ElemType)
		return int64(typ.Len) * roundUp(esize, ealign), ealign
	default:
		return 8, 8 // Default to pointer size
	}
}

func (lcg *LLVMCodeGen) getLLVMParamType(t ortypes.Type) types.Type {
	return lcg.getLLVMTypeFromSemantic(t)
}

func (lcg *LLVMCodeGen) getLLVMReturnType(t ortypes.Type) types.Type {
	llvmType := lcg.getLLVMTypeFromSemantic(t)

	// Resolve lazy
	t = ortypes.LazyResolve(t)

	// If it's a struct type, unwrap the pointer to return by value
	if _, ok := t.(*ortypes.StructType); ok {
		if ptr, ok := llvmType.(*types.PointerType); ok {
			return ptr.ElemType
		}
	}

	// Also check PrimitiveType if it refers to a struct
	if pt, ok := t.(*ortypes.PrimitiveType); ok {
		if _, ok := lcg.structs[pt.Type]; ok {
			if ptr, ok := llvmType.(*types.PointerType); ok {
				return ptr.ElemType
			}
		}
	}

	// Also check pointer to PrimitiveType if it refers to a struct (e.g. *Point)
	// Wait, if it's *Point, getLLVMTypeFromSemantic returns %Point*.
	// We want to return %Point*.
	// So only unwrap if it's NOT a pointer in semantic type.
	// But ortypes.PrimitiveType "Point" means value semantics in Orlang?
	// In Orlang, "Point" is a struct. Variables are references?
	// No, "var p Point" allocates a struct.
	// "fn f() => Point" returns a struct.

	return llvmType
}

func (lcg *LLVMCodeGen) getLLVMTypeFromSemantic(t ortypes.Type) types.Type {
	if t == nil {
		return types.I32
	}

	// Resolve LazyType first
	if lazyType, ok := t.(*ortypes.LazyType); ok {
		resolvedType := lazyType.Resolver()
		return lcg.getLLVMTypeFromSemantic(resolvedType)
	}

	switch t := t.(type) {
	case ortypes.PrimitiveType:
		// Handle value type
		if t.Type == "string" {
			return types.I8Ptr
		}
		if t.Type == "void" {
			return types.Void
		}
		if t.Type == "int64" {
			return types.I64
		}
		if t.Type == "int8" {
			return types.I8
		}
		if t.Type == "float32" {
			return types.Float
		}
		if t.Type == "float64" {
			return types.Double
		}
		if t.Type == "int16" || t.Type == "uint16" {
			return types.I16
		}
		if t.Type == "uint64" {
			return types.I64
		}
		if t.Type == "uint32" {
			return types.I32
		}
		if t.Type == "uint8" {
			return types.I8
		}
		// Check if it's actually a struct type name
		if s, ok := lcg.structs[t.Type]; ok {
			return types.NewPointer(s)
		}
		if t.Type == "bool" {
			return types.I1
		}
		// Default for other primitives (int32, etc.)
		return types.I32
	case *ortypes.PrimitiveType:
		// Handle pointer type
		if t.Type == "string" {
			return types.I8Ptr
		}
		if t.Type == "void" {
			return types.Void
		}
		if t.Type == "int64" {
			return types.I64
		}
		if t.Type == "int8" {
			return types.I8
		}
		if t.Type == "float32" {
			return types.Float
		}
		if t.Type == "float64" {
			return types.Double
		}
		if t.Type == "int16" || t.Type == "uint16" {
			return types.I16
		}
		if t.Type == "uint64" {
			return types.I64
		}
		if t.Type == "uint32" {
			return types.I32
		}
		if t.Type == "uint8" {
			return types.I8
		}
		// Check if it's actually a struct type name
		if s, ok := lcg.structs[t.Type]; ok {
			return types.NewPointer(s)
		}
		// Default for other primitives
		return types.I32
	case *ortypes.StructType:
		if s, ok := lcg.structs[t.Name]; ok {
			return types.NewPointer(s)
		}
	case *ortypes.InterfaceType:
		return lcg.getInterfaceType()
	case *ortypes.TupleType:
		// Map tuple to LLVM struct type
		var fields []types.Type
		for _, elemType := range t.Types {
			fields = append(fields, lcg.getLLVMTypeFromSemantic(elemType))
		}
		return types.NewStruct(fields...)
	case *ortypes.ArrayType:
		elemType := lcg.getLLVMTypeFromSemantic(t.Type)
		if t.Length >= 0 {
			return types.NewArray(uint64(t.Length), elemType)
		}
		// Slice: struct { data *T, len i32 }
		return types.NewStruct(types.NewPointer(elemType), types.I32)
	case *ortypes.PointerType:
		// Handle explicit pointer types like &int8
		innerType := lcg.getLLVMTypeFromSemantic(t.Type)
		return types.NewPointer(innerType)
	case *ortypes.SignatureType:
		// Closure struct: { fn_ptr as i8*, env_ptr as i8* }
		return lcg.getClosureType()
	case *ortypes.MapType:
		// Maps are opaque pointers (i8*) to the C runtime Map struct
		return types.NewPointer(types.I8)
	}

	return types.I32
}

func (lcg *LLVMCodeGen) getLLVMType(t ast.Type) types.Type {
	switch typ := t.(type) {
	case *ast.TypeReference:
		// For type references, we don't know the exact type.
		// We need to look it up.
		name := typ.Name.Text
		if nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[typ]; nodeInfo != nil {
			return lcg.getLLVMTypeFromSemantic(nodeInfo.Type)
		}

		switch name {
		case "int", "int32", "uint32":
			return types.I32
		case "int64", "uint64":
			return types.I64
		case "int16", "uint16":
			return types.I16
		case "int8", "uint8":
			return types.I8
		case "string":
			return types.I8Ptr
		case "bool":
			return types.I1
		case "float", "float32":
			return types.Float
		case "float64":
			return types.Double
			// Add more primitive types here if needed.
		}

		// But getLLVMType takes ast.Type, which is just a name.
		// We can't resolve struct or interface types from ast.Type alone.
		// We need to lookup from semantic info.
		// But we don't always have semantic info here (e.g. for params in function declarations before analysis?)
		// Actually, when we're generating code, we always have semantic info.
		// But getLLVMType is used when we declare variables etc.
		// If we have semantic info, we should prefer getLLVMTypeFromSemantic.
		// For now, let's just return I32 as a fallback.
		// return types.I32
		return types.I8Ptr
	case *ast.PointerType:
		// Get the base type and return a pointer to it
		baseType := lcg.getLLVMType(typ.Type)
		return types.NewPointer(baseType)
	case *ast.ArrayType:
		elemType := lcg.getLLVMType(typ.Type)
		if typ.Length != nil {
			// Fixed array: decay to pointer to element
			return types.NewPointer(elemType)
		}
		// Slice: struct { data *T, len i32 }
		return types.NewStruct(types.NewPointer(elemType), types.I32)
	case *ast.TupleType:
		// Map tuple type to LLVM struct
		var fields []types.Type
		for _, elemType := range typ.Types {
			fields = append(fields, lcg.getLLVMType(elemType))
		}
		return types.NewStruct(fields...)
	}

	return types.I32
}

func (lcg *LLVMCodeGen) visitStructExpression(n *ast.StructExpression) {
	name := n.Identifier.Text
	structType, ok := lcg.structs[name]
	if !ok {
		// Check if it's an imported struct from analyzer info
		// Look up the type of the StructExpression itself, not its Identifier
		if typNode, exists := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n]; exists {
			if structTyp, ok := typNode.Type.(*ortypes.StructType); ok {
				// Create the LLVM struct type from the semantic type
				var fields []types.Type
				fieldIndices := make(map[string]int)
				for i, v := range structTyp.Variables {
					fields = append(fields, lcg.getLLVMTypeFromSemantic(v.Type))
					fieldIndices[v.Name] = i
				}

				llvmStructType := types.NewStruct(fields...)
				typeDef := lcg.module.NewTypeDef(name, llvmStructType)
				lcg.structs[name] = typeDef
				lcg.structDefinitions[name] = llvmStructType
				lcg.structFields[name] = fieldIndices

				structType = typeDef
			} else {
				return
			}
		} else {
			return
		}
	}

	// Allocate struct on the heap (GC-managed)
	mallocFn := lcg.getOrDeclareGCMalloc()
	structSize := lcg.getSizeOf(structType)
	rawPtr := lcg.currentBlock.NewCall(mallocFn, constant.NewInt(types.I64, structSize))
	alloca := lcg.currentBlock.NewBitCast(rawPtr, types.NewPointer(structType))

	// Initialize fields
	fieldIndices := lcg.structFields[name]

	// Handle positional arguments
	// TODO: Handle named arguments properly. For now assuming positional or named matching

	for i, arg := range n.Arguments {
		ast.Walk(lcg, arg.Expression)
		val := lcg.values[arg.Expression]

		var fieldIdx int
		if arg.Name != nil {
			fieldIdx = fieldIndices[arg.Name.Text]
		} else {
			fieldIdx = i
		}

		zero := constant.NewInt(types.I32, 0)
		idx := constant.NewInt(types.I32, int64(fieldIdx))
		gep := lcg.currentBlock.NewGetElementPtr(structType, alloca, zero, idx)
		lcg.currentBlock.NewStore(val, gep)
	}

	lcg.values[n] = alloca
}

func (lcg *LLVMCodeGen) visitMemberExpression(n *ast.MemberExpression) {
	// Check if this is an enum member access (e.g., Color.Red)
	if ident, ok := n.Target.(*ast.Identifier); ok {
		if enumVals, ok := lcg.enumValues[ident.Text]; ok {
			if val, ok := enumVals[n.Property.Text]; ok {
				lcg.values[n] = constant.NewInt(types.I32, int64(val))
				return
			}
		}
	}

	// Get address of target
	addr := lcg.getAddress(n.Target)
	if addr == nil {
		return
	}

	// Get struct type
	// We need to know the type of the target to know field indices
	ptrType, ok := addr.Type().(*types.PointerType)
	if !ok {
		return
	}

	// Check for double indirection
	if elemPtrType, ok := ptrType.ElemType.(*types.PointerType); ok {
		// Load the pointer
		addr = lcg.currentBlock.NewLoad(elemPtrType, addr)
		ptrType = elemPtrType
	}

	// Get struct name from the final element type
	var structName string
	if namedType, ok := ptrType.ElemType.(*types.StructType); ok {
		structName = namedType.Name()
	} else {
		structName = ptrType.ElemType.Name()
	}

	if structName == "" {
		return
	}

	structType := lcg.structDefinitions[structName]
	if structType == nil {
		return
	}

	fieldIndices, ok := lcg.structFields[structName]
	if !ok {
		return
	}

	fieldIdx, ok := fieldIndices[n.Property.Text]
	if !ok {
		return
	}

	zero := constant.NewInt(types.I32, 0)
	idx := constant.NewInt(types.I32, int64(fieldIdx))
	gep := lcg.currentBlock.NewGetElementPtr(ptrType.ElemType, addr, zero, idx)

	// Load value
	load := lcg.currentBlock.NewLoad(structType.Fields[fieldIdx], gep)
	lcg.values[n] = load
}

func (lcg *LLVMCodeGen) visitIndexExpression(n *ast.IndexExpression) {
	// Check if this is a map access by looking at semantic type info
	nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.Target]
	if nodeInfo != nil {
		if mapType, isMapType := nodeInfo.Type.(*ortypes.MapType); isMapType {
			// Handle map get operation
			ast.Walk(lcg, n.Target)
			mapPtr := lcg.values[n.Target]
			if mapPtr == nil {
				return
			}

			// Evaluate key
			ast.Walk(lcg, n.Index)
			keyVal := lcg.values[n.Index]
			if keyVal == nil {
				return
			}

			// Declare map_get function (returns i64 for generic value storage)
			var mapGetFn *ir.Func
			if fn, ok := lcg.functions["map_get"]; ok {
				mapGetFn = fn
			} else {
				mapStructPtr := types.NewPointer(types.I8)
				mapGetFn = lcg.module.NewFunc("map_get", types.I64,
					ir.NewParam("map", mapStructPtr),
					ir.NewParam("key", types.I8Ptr))
				lcg.functions["map_get"] = mapGetFn
			}

			// Call map_get (returns i64)
			rawResult := lcg.currentBlock.NewCall(mapGetFn, mapPtr, keyVal)

			// Convert i64 result to the actual value type
			valType := lcg.getLLVMTypeFromSemantic(mapType.ValueType)
			lcg.values[n] = lcg.convertMapValueFromI64(rawResult, valType)
			return
		}
	}

	// Original array/slice logic
	// Get address of array
	arrayAddr := lcg.getAddress(n.Target)
	if arrayAddr == nil {
		// If getAddress returns nil, try evaluating the target as expression
		ast.Walk(lcg, n.Target)
		arrayAddr = lcg.values[n.Target]
		if arrayAddr == nil {
			return
		}
	}

	// Evaluate the index expression
	ast.Walk(lcg, n.Index)
	index := lcg.values[n.Index]
	if index == nil {
		return
	}

	// Get the array type from the pointer
	ptrType, ok := arrayAddr.Type().(*types.PointerType)
	if !ok {
		return
	}

	// Check if we have a pointer to pointer (double indirection)
	// This happens when we get the address of a variable that holds an array
	if innerPtrType, ok := ptrType.ElemType.(*types.PointerType); ok {
		// We have PointerType -> PointerType -> ArrayType (or PointerType for parameters)
		// We need to load once to get PointerType -> ArrayType or PointerType
		arrayAddr = lcg.currentBlock.NewLoad(innerPtrType, arrayAddr)
		ptrType = innerPtrType
	}

	// Check if the element type is an array or a pointer
	if arrayType, ok := ptrType.ElemType.(*types.ArrayType); ok {
		// It's an actual array type - use GEP with two indices
		zero := constant.NewInt(types.I32, 0)
		gep := lcg.currentBlock.NewGetElementPtr(arrayType, arrayAddr, zero, index)
		load := lcg.currentBlock.NewLoad(arrayType.ElemType, gep)
		lcg.values[n] = load
	} else if elemPtrType, ok := ptrType.ElemType.(*types.PointerType); ok {
		// It's a pointer type (e.g., function parameter i8**)
		// arrayAddr is i8***, ptrType.ElemType is i8**
		// GEP to get address of arr[index] (which is i8**)
		gep := lcg.currentBlock.NewGetElementPtr(ptrType.ElemType, arrayAddr, index)
		// Load the element (an i8*)
		load := lcg.currentBlock.NewLoad(elemPtrType, gep)
		lcg.values[n] = load
	} else if structType, ok := ptrType.ElemType.(*types.StructType); ok {
		// Slice struct { data*, len }
		// We need to extract the data pointer (index 0)

		zero := constant.NewInt(types.I32, 0)

		// GEP to get pointer to data field
		dataFieldPtr := lcg.currentBlock.NewGetElementPtr(structType, arrayAddr, zero, zero)

		// Load data pointer
		dataPtr := lcg.currentBlock.NewLoad(structType.Fields[0], dataFieldPtr)

		// GEP on data pointer with index
		gep := lcg.currentBlock.NewGetElementPtr(structType.Fields[0].(*types.PointerType).ElemType, dataPtr, index)

		// Load element
		load := lcg.currentBlock.NewLoad(structType.Fields[0].(*types.PointerType).ElemType, gep)
		lcg.values[n] = load
	} else if ptrType.ElemType.Equal(types.I8) {
		// String (i8*) — index into character array
		gep := lcg.currentBlock.NewGetElementPtr(types.I8, arrayAddr, index)
		load := lcg.currentBlock.NewLoad(types.I8, gep)
		// Zero-extend i8 to i32 for use in expressions
		ext := lcg.currentBlock.NewZExt(load, types.I32)
		lcg.values[n] = ext
	} else {
		return
	}
}

func (lcg *LLVMCodeGen) visitArrayExpression(n *ast.ArrayExpression) {
	// Get the array type from analyzer info
	nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n]
	if nodeInfo == nil || nodeInfo.Type == nil {
		return
	}

	arrayType, ok := nodeInfo.Type.(*ortypes.ArrayType)
	if !ok {
		return
	}

	// Convert to LLVM type
	elementType := lcg.getLLVMTypeFromSemantic(arrayType.Type)
	var arrayLength int64

	// Determine array length
	if n.Type.Length != nil {
		// Fixed size array: [3]string{}
		ast.Walk(lcg, n.Type.Length)
		lengthVal := lcg.values[n.Type.Length]
		if constInt, ok := lengthVal.(*constant.Int); ok {
			arrayLength = constInt.X.Int64()
		}
	} else {
		// Inferred size from elements: []string{"a", "b"}
		arrayLength = int64(len(n.Expressions))
	}

	llvmArrayType := types.NewArray(uint64(arrayLength), elementType)

	// Allocate array on stack
	alloca := lcg.currentBlock.NewAlloca(llvmArrayType)

	// Initialize elements
	zero := constant.NewInt(types.I32, 0)

	if len(n.Expressions) == 0 {
		// Zero initialize if no expressions provided (e.g. [3]int{})
		// We can use memset or a loop. For simplicity, loop.
		// Actually, if we just want zero init, we can iterate up to arrayLength
		for i := int64(0); i < arrayLength; i++ {
			idx := constant.NewInt(types.I32, i)
			gep := lcg.currentBlock.NewGetElementPtr(llvmArrayType, alloca, zero, idx)

			// Store default value
			if elementType.Equal(types.I8Ptr) {
				// String: initialize to ""
				emptyStr := lcg.addStringConstant("")
				lcg.currentBlock.NewStore(emptyStr, gep)
			} else {
				// Zero initialize
				lcg.currentBlock.NewStore(constant.NewZeroInitializer(elementType), gep)
			}
		}
	} else {
		for i, expr := range n.Expressions {
			// Evaluate expression
			ast.Walk(lcg, expr)
			val := lcg.values[expr]
			if val == nil {
				continue
			}

			// Get pointer to element
			idx := constant.NewInt(types.I32, int64(i))
			gep := lcg.currentBlock.NewGetElementPtr(llvmArrayType, alloca, zero, idx)

			// Store value
			lcg.currentBlock.NewStore(val, gep)
		}
	}

	// Store the result
	// Use semantic type to determine if this is a fixed array or slice
	// because n.Type.Length might not be set correctly by the parser
	isFixedArray := arrayType.Length >= 0

	if !isFixedArray {
		// Slice: create struct { data *T, len i32 }
		sliceType := types.NewStruct(types.NewPointer(elementType), types.I32)

		// Create struct value
		var sliceVal value.Value = constant.NewStruct(sliceType, constant.NewNull(types.NewPointer(elementType)), constant.NewInt(types.I32, 0))

		// Decay array to pointer
		dataPtr := lcg.currentBlock.NewGetElementPtr(llvmArrayType, alloca, zero, zero)

		// Insert data pointer
		sliceVal = lcg.currentBlock.NewInsertValue(sliceVal, dataPtr, 0)

		// Insert length
		lenVal := constant.NewInt(types.I32, arrayLength)
		sliceVal = lcg.currentBlock.NewInsertValue(sliceVal, lenVal, 1)

		lcg.values[n] = sliceVal
	} else {
		// Fixed array: return pointer to array
		lcg.values[n] = alloca
	}
}

func (lcg *LLVMCodeGen) visitMapExpression(n *ast.MapExpression) {
	// Declare map_create function if not already declared
	var mapCreateFn *ir.Func
	if fn, ok := lcg.functions["map_create"]; ok {
		mapCreateFn = fn
	} else {
		// map_create returns a pointer to the Map struct (opaque)
		mapStructPtr := types.NewPointer(types.I8) // Use i8* for opaque pointer
		mapCreateFn = lcg.module.NewFunc("map_create", mapStructPtr)
		lcg.functions["map_create"] = mapCreateFn
	}

	// Create the map
	mapPtr := lcg.currentBlock.NewCall(mapCreateFn)

	// Insert each entry
	if len(n.Entries) > 0 {
		// Declare map_insert function (takes i64 for generic value storage)
		mapInsertFn := lcg.getMapInsertFn()

		for _, entry := range n.Entries {
			// Evaluate key (should be a string)
			ast.Walk(lcg, entry.Key)
			keyVal := lcg.values[entry.Key]

			// Evaluate value
			ast.Walk(lcg, entry.Value)
			valueVal := lcg.values[entry.Value]

			// Convert value to i64 for storage
			i64Val := lcg.convertMapValueToI64(valueVal)

			// Call map_insert
			lcg.currentBlock.NewCall(mapInsertFn, mapPtr, keyVal, i64Val)
		}
	}

	// Store the map pointer as the result
	lcg.values[n] = mapPtr
}

// getMapInsertFn returns (or declares) the map_insert function.
// map_insert takes i64 values for generic storage.
func (lcg *LLVMCodeGen) getMapInsertFn() *ir.Func {
	if fn, ok := lcg.functions["map_insert"]; ok {
		return fn
	}
	mapStructPtr := types.NewPointer(types.I8)
	fn := lcg.module.NewFunc("map_insert", types.Void,
		ir.NewParam("map", mapStructPtr),
		ir.NewParam("key", types.I8Ptr),
		ir.NewParam("value", types.I64))
	lcg.functions["map_insert"] = fn
	return fn
}

// convertMapValueToI64 converts a value to i64 for map storage.
func (lcg *LLVMCodeGen) convertMapValueToI64(val value.Value) value.Value {
	valType := val.Type()
	if valType.Equal(types.I64) {
		return val
	}
	if _, ok := valType.(*types.IntType); ok {
		// Sign-extend smaller integers to i64
		return lcg.currentBlock.NewSExt(val, types.I64)
	}
	if _, ok := valType.(*types.PointerType); ok {
		// Pointer (e.g., string i8*) → ptrtoint i64
		return lcg.currentBlock.NewPtrToInt(val, types.I64)
	}
	// Fallback: try sext (covers i1/bool etc)
	return lcg.currentBlock.NewSExt(val, types.I64)
}

// convertMapValueFromI64 converts an i64 from map storage to the target type.
func (lcg *LLVMCodeGen) convertMapValueFromI64(val value.Value, targetType types.Type) value.Value {
	if targetType.Equal(types.I64) {
		return val
	}
	if intType, ok := targetType.(*types.IntType); ok {
		// Truncate i64 to smaller integer
		return lcg.currentBlock.NewTrunc(val, intType)
	}
	if _, ok := targetType.(*types.PointerType); ok {
		// i64 → pointer (e.g., string i8*)
		return lcg.currentBlock.NewIntToPtr(val, targetType)
	}
	// Fallback: truncate to i32
	return lcg.currentBlock.NewTrunc(val, types.I32)
}

func (lcg *LLVMCodeGen) getAddress(n ast.Node) value.Value {
	if ident, ok := n.(*ast.Identifier); ok {
		nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[ident]
		if nodeInfo != nil {
			details := nodeInfo.Scope.GetDetails(ident.Text, true)
			if details != nil {
				if val, ok := lcg.values[details.DefineIdentifier]; ok {
					return val
				}
			}
		}
		// Fallback: check if the value was stored under this identifier directly
		if val, ok := lcg.values[n]; ok {
			return val
		}
		// Check if it's 'this'
		if ident.Text == "this" {
			// We need to find the 'this' parameter
			// It's the first parameter of the current function if we are in a method
			if lcg.currentStruct != nil {
				return lcg.currentFunc.Params[0]
			}
		}
	} else if member, ok := n.(*ast.MemberExpression); ok {
		// Recursive getAddress for nested members
		targetAddr := lcg.getAddress(member.Target)
		if targetAddr == nil {
			return nil
		}

		ptrType, ok := targetAddr.Type().(*types.PointerType)
		if !ok {
			return nil
		}

		// Check for double indirection
		if elemPtrType, ok := ptrType.ElemType.(*types.PointerType); ok {
			// Load the pointer
			targetAddr = lcg.currentBlock.NewLoad(elemPtrType, targetAddr)
			ptrType = elemPtrType
		}

		structName := ptrType.ElemType.Name()
		if structName == "" {
			return nil
		}

		structType := lcg.structDefinitions[structName]
		if structType == nil {
			return nil
		}

		fieldIndices, ok := lcg.structFields[structName]
		if !ok {
			return nil
		}

		fieldIdx, ok := fieldIndices[member.Property.Text]
		if !ok {
			return nil
		}

		zero := constant.NewInt(types.I32, 0)
		idx := constant.NewInt(types.I32, int64(fieldIdx))
		gep := lcg.currentBlock.NewGetElementPtr(ptrType.ElemType, targetAddr, zero, idx)
		return gep
	} else if indexExpr, ok := n.(*ast.IndexExpression); ok {
		// Get address of array
		arrayAddr := lcg.getAddress(indexExpr.Target)
		if arrayAddr == nil {
			return nil
		}

		// Evaluate index
		ast.Walk(lcg, indexExpr.Index)
		index := lcg.values[indexExpr.Index]
		if index == nil {
			return nil
		}

		ptrType, ok := arrayAddr.Type().(*types.PointerType)
		if !ok {
			return nil
		}

		// Check for double indirection
		if innerPtrType, ok := ptrType.ElemType.(*types.PointerType); ok {
			arrayAddr = lcg.currentBlock.NewLoad(innerPtrType, arrayAddr)
			ptrType = innerPtrType
		}

		if arrayType, ok := ptrType.ElemType.(*types.ArrayType); ok {
			// Fixed array
			zero := constant.NewInt(types.I32, 0)
			gep := lcg.currentBlock.NewGetElementPtr(arrayType, arrayAddr, zero, index)
			return gep
		} else if elemPtrType, ok := ptrType.ElemType.(*types.PointerType); ok {
			// Pointer (parameter)
			pointerVal := lcg.currentBlock.NewLoad(ptrType.ElemType, arrayAddr)
			gep := lcg.currentBlock.NewGetElementPtr(elemPtrType.ElemType, pointerVal, index)
			return gep
		} else if structType, ok := ptrType.ElemType.(*types.StructType); ok {
			// Slice struct
			zero := constant.NewInt(types.I32, 0)
			dataFieldPtr := lcg.currentBlock.NewGetElementPtr(structType, arrayAddr, zero, zero)
			dataPtr := lcg.currentBlock.NewLoad(structType.Fields[0], dataFieldPtr)
			gep := lcg.currentBlock.NewGetElementPtr(structType.Fields[0].(*types.PointerType).ElemType, dataPtr, index)
			return gep
		}
	}
	return nil
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
	case *ast.ComparisonExpression:
		lcg.visitComparisonExpression(n)
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
	case *ast.IfStatement:
		lcg.visitIfStatement(n)
		return nil
	case *ast.ImportStatement:
		lcg.visitImportStatement(n)
		return nil
	case *ast.ExportStatement:
		lcg.visitExportStatement(n)
		return nil
	case *ast.IncludeStatement:
		lcg.visitIncludeStatement(n)
		return nil
	case *ast.LinkStatement:
		// Link directives are handled by the compile pipeline
		return nil
	case *ast.Struct:
		lcg.visitStruct(n)
		return nil
	case *ast.Enum:
		lcg.visitEnum(n)
		return nil
	case *ast.StructExpression:
		lcg.visitStructExpression(n)
		return nil
	case *ast.MemberExpression:
		lcg.visitMemberExpression(n)
		return nil
	case *ast.TupleExpression:
		lcg.visitTupleExpression(n)
		return nil
	case *ast.TupleDeclaration:
		lcg.visitTupleDeclaration(n)
		return nil
	case *ast.TypeAssertionExpression:
		lcg.visitTypeAssertionExpression(n)
		return nil
	case *ast.CastExpression:
		lcg.visitCastExpression(n)
		return nil
	case *ast.ArrayExpression:
		lcg.visitArrayExpression(n)
		return nil
	case *ast.MapExpression:
		lcg.visitMapExpression(n)
		return nil
	case *ast.IndexExpression:
		lcg.visitIndexExpression(n)
		return nil
	case *ast.ForLoop:
		lcg.visitForLoop(n)
		return nil
	case *ast.ForRangeLoop:
		lcg.visitForRangeLoop(n)
		return nil
	case *ast.UnaryExpression:
		lcg.visitUnaryExpression(n)
		return nil
	case *ast.BreakStatement:
		if len(lcg.loopExitBlocks) > 0 {
			lcg.currentBlock.NewBr(lcg.loopExitBlocks[len(lcg.loopExitBlocks)-1])
			lcg.currentBlock = lcg.currentFunc.NewBlock("")
		}
		return nil
	case *ast.ContinueStatement:
		if len(lcg.loopPostBlocks) > 0 {
			lcg.currentBlock.NewBr(lcg.loopPostBlocks[len(lcg.loopPostBlocks)-1])
			lcg.currentBlock = lcg.currentFunc.NewBlock("")
		}
		return nil
	case *ast.SwitchStatement:
		lcg.visitSwitchStatement(n)
		return nil
	case *ast.DeferStatement:
		lcg.visitDeferStatement(n)
		return nil
	}
	return lcg
}

func (lcg *LLVMCodeGen) visitUnaryExpression(n *ast.UnaryExpression) {
	// Handle postfix increment/decrement
	if n.Postfix {
		// Get address of operand
		addr := lcg.getAddress(n.Expression)
		if addr == nil {
			return
		}

		// Load current value
		val := lcg.currentBlock.NewLoad(addr.Type().(*types.PointerType).ElemType, addr)

		// Store original value as result of expression
		lcg.values[n] = val

		// Calculate new value
		var newVal value.Value
		one := constant.NewInt(val.Type().(*types.IntType), 1)

		switch n.Operator.Type {
		case scanner.TokenTypeIncrement:
			newVal = lcg.currentBlock.NewAdd(val, one)
		case scanner.TokenTypeDecrement:
			newVal = lcg.currentBlock.NewSub(val, one)
		default:
			return
		}

		// Store new value back to address
		lcg.currentBlock.NewStore(newVal, addr)
		return
	}

	// Handle prefix unary expressions
	ast.Walk(lcg, n.Expression)
	operand := lcg.values[n.Expression]
	if operand == nil {
		return
	}

	switch n.Operator.Type {
	case scanner.TokenTypeEXCL:
		// Logical NOT: !expr
		boolVal := lcg.ensureBool(operand)
		lcg.values[n] = lcg.currentBlock.NewXor(boolVal, constant.True)
	case scanner.TokenTypeSUB:
		// Unary minus: -expr
		if intType, ok := operand.Type().(*types.IntType); ok {
			lcg.values[n] = lcg.currentBlock.NewSub(constant.NewInt(intType, 0), operand)
		} else if floatType, ok := operand.Type().(*types.FloatType); ok {
			lcg.values[n] = lcg.currentBlock.NewFSub(constant.NewFloat(floatType, 0), operand)
		} else {
			lcg.errorf(n, "unary minus is not defined for this operand type")
		}
	case scanner.TokenTypeIncrement:
		// Prefix ++
		addr := lcg.getAddress(n.Expression)
		if addr == nil {
			return
		}
		val := lcg.currentBlock.NewLoad(addr.Type().(*types.PointerType).ElemType, addr)
		one := constant.NewInt(val.Type().(*types.IntType), 1)
		newVal := lcg.currentBlock.NewAdd(val, one)
		lcg.currentBlock.NewStore(newVal, addr)
		lcg.values[n] = newVal
	case scanner.TokenTypeDecrement:
		// Prefix --
		addr := lcg.getAddress(n.Expression)
		if addr == nil {
			return
		}
		val := lcg.currentBlock.NewLoad(addr.Type().(*types.PointerType).ElemType, addr)
		one := constant.NewInt(val.Type().(*types.IntType), 1)
		newVal := lcg.currentBlock.NewSub(val, one)
		lcg.currentBlock.NewStore(newVal, addr)
		lcg.values[n] = newVal
	case scanner.TokenTypeTILDE:
		// Bitwise NOT: ~expr (XOR with all ones = -1)
		if intType, ok := operand.Type().(*types.IntType); ok {
			lcg.values[n] = lcg.currentBlock.NewXor(operand, constant.NewInt(intType, -1))
		}
	}
}

func (lcg *LLVMCodeGen) visitForLoop(n *ast.ForLoop) {
	// 1. Create blocks
	// condBlock: check condition
	// bodyBlock: loop body
	// postBlock: increment/post statement
	// afterBlock: exit loop

	condBlock := lcg.currentFunc.NewBlock("")
	bodyBlock := lcg.currentFunc.NewBlock("")
	postBlock := lcg.currentFunc.NewBlock("")
	afterBlock := lcg.currentFunc.NewBlock("")

	// 2. Execute Init statement
	if n.Init != nil {
		ast.Walk(lcg, n.Init)
	}

	// Jump to condition check
	lcg.currentBlock.NewBr(condBlock)

	// 3. Condition Block
	lcg.currentBlock = condBlock
	if n.Condition != nil {
		ast.Walk(lcg, n.Condition)
		condVal := lcg.values[n.Condition]

		// Ensure boolean
		if condVal.Type().Equal(types.I1) {
			lcg.currentBlock.NewCondBr(condVal, bodyBlock, afterBlock)
		} else {
			// Try to cast to bool or compare with 0
			// For now assume it's boolean-like or error
			// If it's int, compare != 0
			zero := constant.NewInt(condVal.Type().(*types.IntType), 0)
			condBool := lcg.currentBlock.NewICmp(enum.IPredNE, condVal, zero)
			lcg.currentBlock.NewCondBr(condBool, bodyBlock, afterBlock)
		}
	} else {
		// Infinite loop
		lcg.currentBlock.NewBr(bodyBlock)
	}

	// 4. Body Block
	lcg.currentBlock = bodyBlock
	lcg.loopExitBlocks = append(lcg.loopExitBlocks, afterBlock)
	lcg.loopPostBlocks = append(lcg.loopPostBlocks, postBlock)
	ast.Walk(lcg, n.Block)
	lcg.loopExitBlocks = lcg.loopExitBlocks[:len(lcg.loopExitBlocks)-1]
	lcg.loopPostBlocks = lcg.loopPostBlocks[:len(lcg.loopPostBlocks)-1]

	// If body doesn't terminate, jump to post
	if !lcg.isTerminator(lcg.currentBlock.Term) {
		lcg.currentBlock.NewBr(postBlock)
	}

	// 5. Post Block
	lcg.currentBlock = postBlock
	if n.After != nil {
		ast.Walk(lcg, n.After)
	}
	// Jump back to condition
	lcg.currentBlock.NewBr(condBlock)

	// 6. After Block
	lcg.currentBlock = afterBlock
}

func (lcg *LLVMCodeGen) visitForRangeLoop(n *ast.ForRangeLoop) {
	// 1. Evaluate iterable
	ast.Walk(lcg, n.Iterable)
	iterVal := lcg.values[n.Iterable]
	if iterVal == nil {
		return
	}

	// 2. Determine element type, length, and data pointer from the iterable value
	var elemType types.Type
	var length value.Value
	var dataPtr value.Value

	iterType := iterVal.Type()
	if structType, ok := iterType.(*types.StructType); ok {
		// Slice struct { T*, i32 }
		if len(structType.Fields) >= 2 {
			length = lcg.currentBlock.NewExtractValue(iterVal, 1)
			dataPtr = lcg.currentBlock.NewExtractValue(iterVal, 0)
			if ptrType, ok := structType.Fields[0].(*types.PointerType); ok {
				elemType = ptrType.ElemType
			}
		}
	} else if ptrType, ok := iterType.(*types.PointerType); ok {
		// Decayed array pointer T* (fixed-size array)
		elemType = ptrType.ElemType
		dataPtr = iterVal
		// Get length from semantic type info
		nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.Iterable]
		if nodeInfo != nil {
			if arrType, ok := nodeInfo.Type.(*ortypes.ArrayType); ok {
				length = constant.NewInt(types.I32, arrType.Length)
			}
		}
	}

	if elemType == nil || length == nil || dataPtr == nil {
		return
	}

	// 3. Create index alloca, initialize to 0
	idxAlloca := lcg.currentBlock.NewAlloca(types.I32)
	lcg.currentBlock.NewStore(constant.NewInt(types.I32, 0), idxAlloca)

	// 4. Create blocks
	condBlock := lcg.currentFunc.NewBlock("")
	bodyBlock := lcg.currentFunc.NewBlock("")
	postBlock := lcg.currentFunc.NewBlock("")
	afterBlock := lcg.currentFunc.NewBlock("")

	lcg.currentBlock.NewBr(condBlock)

	// 5. Condition block: idx < length
	lcg.currentBlock = condBlock
	idx := lcg.currentBlock.NewLoad(types.I32, idxAlloca)
	cond := lcg.currentBlock.NewICmp(enum.IPredSLT, idx, length)
	lcg.currentBlock.NewCondBr(cond, bodyBlock, afterBlock)

	// 6. Body block
	lcg.currentBlock = bodyBlock

	// Load current index
	bodyIdx := lcg.currentBlock.NewLoad(types.I32, idxAlloca)

	// Get element: GEP + load
	elemPtr := lcg.currentBlock.NewGetElementPtr(elemType, dataPtr, bodyIdx)
	elemVal := lcg.currentBlock.NewLoad(elemType, elemPtr)

	// Create value variable alloca and store element
	valAlloca := lcg.currentBlock.NewAlloca(elemType)
	lcg.currentBlock.NewStore(elemVal, valAlloca)
	lcg.values[n.ValueName] = valAlloca

	// If index name provided, create alloca for it
	if n.IndexName != nil {
		idxVarAlloca := lcg.currentBlock.NewAlloca(types.I32)
		lcg.currentBlock.NewStore(bodyIdx, idxVarAlloca)
		lcg.values[n.IndexName] = idxVarAlloca
	}

	// Push loop blocks for break/continue
	lcg.loopExitBlocks = append(lcg.loopExitBlocks, afterBlock)
	lcg.loopPostBlocks = append(lcg.loopPostBlocks, postBlock)

	// Walk body
	ast.Walk(lcg, n.Block)

	lcg.loopExitBlocks = lcg.loopExitBlocks[:len(lcg.loopExitBlocks)-1]
	lcg.loopPostBlocks = lcg.loopPostBlocks[:len(lcg.loopPostBlocks)-1]

	if !lcg.isTerminator(lcg.currentBlock.Term) {
		lcg.currentBlock.NewBr(postBlock)
	}

	// 7. Post block: increment index
	lcg.currentBlock = postBlock
	postIdx := lcg.currentBlock.NewLoad(types.I32, idxAlloca)
	newIdx := lcg.currentBlock.NewAdd(postIdx, constant.NewInt(types.I32, 1))
	lcg.currentBlock.NewStore(newIdx, idxAlloca)
	lcg.currentBlock.NewBr(condBlock)

	// 8. After block
	lcg.currentBlock = afterBlock
}

func (lcg *LLVMCodeGen) visitVariableDeclaration(n *ast.VariableDeclaration) {
	// Global variable: we're at file level (not inside any function)
	if lcg.currentFunc == nil {
		lcg.visitGlobalVariableDeclaration(n)
		return
	}

	var typ types.Type

	// 1. Try explicit type
	if n.Type != nil {
		typ = lcg.getLLVMType(n.Type)
	}

	// 2. Try semantic type from analyser
	if typ == nil {
		nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.Name]
		if nodeInfo != nil && nodeInfo.Type != nil {
			typ = lcg.getLLVMTypeFromSemantic(nodeInfo.Type)
		}
	}

	var val value.Value

	// 3. If still nil, and we have default value, visit it to get type
	if n.DefaultValue != nil {
		ast.Walk(lcg, n.DefaultValue)
		val = lcg.values[n.DefaultValue]

		// If the value is already a struct/array pointer (from alloca or GC_malloc+bitcast),
		// just use it directly instead of creating a new alloca
		var directPtr bool
		var elemType types.Type
		if allocaInst, isAlloca := val.(*ir.InstAlloca); isAlloca {
			directPtr = true
			elemType = allocaInst.ElemType
		} else if ptrType, isPtr := val.Type().(*types.PointerType); isPtr {
			if _, isStruct := ptrType.ElemType.(*types.StructType); isStruct {
				_, isBitCast := val.(*ir.InstBitCast)
				_, isGEP := val.(*ir.InstGetElementPtr)
				_, isCall := val.(*ir.InstCall)
				_, isLoad := val.(*ir.InstLoad)
				if isBitCast || isGEP || isCall || isLoad {
					directPtr = true
					elemType = ptrType.ElemType
				}
			}
		}
		if directPtr {
			lcg.values[n.Name] = val

			// Store metadata
			var semType ortypes.Type
			nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.Name]
			if nodeInfo != nil {
				semType = nodeInfo.Type
			}
			if semType == nil && n.DefaultValue != nil {
				defaultNodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.DefaultValue]
				if defaultNodeInfo != nil {
					semType = defaultNodeInfo.Type
				}
			}
			category := GetTypeCategory(semType, elemType)

			lcg.allocaMetadata[val] = &AllocaMetadata{
				SemanticType: semType,
				Category:     category,
			}

			return
		}

		if typ == nil || typ == types.I32 { // I32 is fallback in getLLVMTypeFromSemantic
			// Use value type
			// But we need to be careful. val.Type() returns LLVM type.
			// If val is a pointer to struct, we want that.
			if val != nil {
				typ = val.Type()
			}
		}
	}

	if typ == nil {
		typ = types.I32 // Fallback
	}

	// Allocate
	alloca := lcg.currentBlock.NewAlloca(typ)
	lcg.values[n.Name] = alloca

	// Store metadata for this alloca
	var semType ortypes.Type
	nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.Name]
	if nodeInfo != nil {
		semType = nodeInfo.Type
	}
	// If we don't have type from Name, try DefaultValue
	if semType == nil && n.DefaultValue != nil {
		defaultNodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.DefaultValue]
		if defaultNodeInfo != nil {
			semType = defaultNodeInfo.Type
		}
	}
	category := GetTypeCategory(semType, typ)
	lcg.allocaMetadata[alloca] = &AllocaMetadata{
		SemanticType: semType,
		Category:     category,
	}

	// Store default value
	if val != nil {
		// Check if we need to cast
		var sourceTyp ortypes.Type
		if n.DefaultValue != nil {
			nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.DefaultValue]
			if nodeInfo != nil {
				sourceTyp = nodeInfo.Type
			}
		}

		var targetTyp ortypes.Type
		if n.Type != nil {
			nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.Type]
			if nodeInfo != nil {
				targetTyp = nodeInfo.Type
			}
		} else {
			// If implicit type, target is same as source (inferred)
			targetTyp = sourceTyp
		}

		// If we inferred type from analyzer earlier (when typ was nil), we should use that.
		// But here we need semantic types to check for interface.
		// If n.Type is nil, we inferred from semantic info of n.Name?
		if targetTyp == nil {
			nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.Name]
			if nodeInfo != nil {
				targetTyp = nodeInfo.Type
			}
		}

		val = lcg.castIfNeeded(val, sourceTyp, targetTyp)
		lcg.currentBlock.NewStore(val, alloca)
	}
}

func (lcg *LLVMCodeGen) visitGlobalVariableDeclaration(n *ast.VariableDeclaration) {
	var typ types.Type

	// Determine type from explicit annotation or analyser
	if n.Type != nil {
		// For globals, use actual array type (not pointer-decayed)
		if arrType, isArray := n.Type.(*ast.ArrayType); isArray && arrType.Length != nil {
			elemType := lcg.getLLVMType(arrType.Type)
			if lenExpr, ok := arrType.Length.(*ast.ValueExpression); ok {
				if length, ok := lenExpr.Token.Value.(int64); ok {
					typ = types.NewArray(uint64(length), elemType)
				}
			}
		}
		if typ == nil {
			typ = lcg.getLLVMType(n.Type)
		}
	}
	if typ == nil {
		nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.Name]
		if nodeInfo != nil && nodeInfo.Type != nil {
			typ = lcg.getLLVMTypeFromSemantic(nodeInfo.Type)
		}
	}
	if typ == nil {
		typ = types.I32
	}

	// Determine the constant initializer
	var init constant.Constant
	if n.DefaultValue != nil {
		if valExpr, ok := n.DefaultValue.(*ast.ValueExpression); ok {
			init = lcg.getConstantFromValue(valExpr, typ)
		}
	}
	if init == nil {
		init = lcg.getZeroValue(typ)
	}

	name := n.Name.Text
	if lcg.moduleName != "" && lcg.moduleName != "main" {
		name = lcg.moduleName + "__" + name
	}

	g := lcg.module.NewGlobalDef(name, init)
	lcg.values[n.Name] = g

	// Store metadata
	var semType ortypes.Type
	nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.Name]
	if nodeInfo != nil {
		semType = nodeInfo.Type
	}
	lcg.allocaMetadata[g] = &AllocaMetadata{
		SemanticType: semType,
		Category:     GetTypeCategory(semType, typ),
	}
}

func (lcg *LLVMCodeGen) getConstantFromValue(valExpr *ast.ValueExpression, typ types.Type) constant.Constant {
	if valExpr.Token.Value == nil {
		return nil
	}
	if i, ok := valExpr.Token.Value.(int64); ok {
		if intType, ok := typ.(*types.IntType); ok {
			return constant.NewInt(intType, i)
		}
		if floatType, ok := typ.(*types.FloatType); ok {
			return constant.NewFloat(floatType, float64(i))
		}
	}
	if f, ok := valExpr.Token.Value.(float64); ok {
		if floatType, ok := typ.(*types.FloatType); ok {
			return constant.NewFloat(floatType, f)
		}
	}
	if b, ok := valExpr.Token.Value.(bool); ok {
		intVal := int64(0)
		if b {
			intVal = 1
		}
		return constant.NewInt(types.I1, intVal)
	}
	return nil
}

func (lcg *LLVMCodeGen) getZeroValue(typ types.Type) constant.Constant {
	switch t := typ.(type) {
	case *types.IntType:
		return constant.NewInt(t, 0)
	case *types.FloatType:
		return constant.NewFloat(t, 0)
	default:
		return constant.NewZeroInitializer(typ)
	}
}

func (lcg *LLVMCodeGen) visitAssigment(n *ast.Assigment) {
	// Check if this is a map assignment (m[key] = value)
	if indexExpr, ok := n.Left.(*ast.IndexExpression); ok {
		nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[indexExpr.Target]
		if nodeInfo != nil {
			if _, isMapType := nodeInfo.Type.(*ortypes.MapType); isMapType {
				// Handle map insert operation
				ast.Walk(lcg, indexExpr.Target)
				mapPtr := lcg.values[indexExpr.Target]
				if mapPtr == nil {
					return
				}

				// Evaluate key
				ast.Walk(lcg, indexExpr.Index)
				keyVal := lcg.values[indexExpr.Index]
				if keyVal == nil {
					return
				}

				// Evaluate value
				ast.Walk(lcg, n.Right)
				valueVal := lcg.values[n.Right]
				if valueVal == nil {
					return
				}

				// Convert value to i64 for storage
				i64Val := lcg.convertMapValueToI64(valueVal)

				// Call map_insert
				mapInsertFn := lcg.getMapInsertFn()
				lcg.currentBlock.NewCall(mapInsertFn, mapPtr, keyVal, i64Val)
				return
			}
		}
	}

	// Original assignment logic
	ast.Walk(lcg, n.Right)
	val := lcg.values[n.Right]

	// Get address of left side
	addr := lcg.getAddress(n.Left)

	// Cast if needed
	var sourceTyp ortypes.Type
	nodeInfoVal := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.Right]
	if nodeInfoVal != nil {
		sourceTyp = nodeInfoVal.Type
	}

	var targetTyp ortypes.Type
	nodeInfoLeft := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n.Left]
	if nodeInfoLeft != nil {
		targetTyp = nodeInfoLeft.Type
	}

	val = lcg.castIfNeeded(val, sourceTyp, targetTyp)

	if val == nil {
		lcg.errorf(n, "cannot generate code for assignment value")
		return
	}
	if addr == nil {
		lcg.errorf(n, "cannot assign: left-hand side is not addressable")
		return
	}

	lcg.currentBlock.NewStore(val, addr)
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

	if leftVal == nil || rightVal == nil {
		return
	}

	var val value.Value

	// Check for operator overloading
	nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n]
	if nodeInfo != nil && nodeInfo.OverloadedOperation != nil {
		// Get struct name from left operand type
		var structName string
		if ptrType, ok := leftVal.Type().(*types.PointerType); ok {
			if namedType, ok := ptrType.ElemType.(*types.StructType); ok {
				structName = namedType.Name()
			} else {
				structName = ptrType.ElemType.Name()
			}
		} else if namedType, ok := leftVal.Type().(*types.StructType); ok {
			structName = namedType.Name()
		}

		if structName != "" {
			fnName := structName + "_op_" + n.Operator.Text
			if fn, ok := lcg.functions[fnName]; ok {
				// Ensure operands are pointers (operator functions take pointer args)
				ensurePtr := func(v value.Value) value.Value {
					if _, isPtr := v.Type().(*types.PointerType); !isPtr {
						alloca := lcg.currentBlock.NewAlloca(v.Type())
						lcg.currentBlock.NewStore(v, alloca)
						return alloca
					}
					return v
				}
				leftPtr := ensurePtr(leftVal)
				rightPtr := ensurePtr(rightVal)
				val = lcg.currentBlock.NewCall(fn, leftPtr, leftPtr, rightPtr)
				lcg.values[n] = val
				return
			}
		}
	}

	// String operations
	if ptrL, ok := leftVal.Type().(*types.PointerType); ok && ptrL.ElemType.Equal(types.I8) {
		if n.Operator.Text == "+" {
			// string + string = concatenation
			if ptrR, ok := rightVal.Type().(*types.PointerType); ok && ptrR.ElemType.Equal(types.I8) {
				lcg.visitStringConcat(n, leftVal, rightVal)
				return
			}
			// string + int = substring offset (pointer arithmetic)
			if _, ok := rightVal.Type().(*types.IntType); ok {
				gep := lcg.currentBlock.NewGetElementPtr(types.I8, leftVal, rightVal)
				lcg.values[n] = gep
				return
			}
		}
	}

	isFloat := isFloatLLVMType(leftVal.Type()) || isFloatLLVMType(rightVal.Type())
	leftVal, rightVal = lcg.unifyNumericOperands(leftVal, rightVal, n.Left, n.Right)
	isUnsigned := lcg.operandsUnsigned(n.Left, n.Right)

	switch n.Operator.Text {
	case "+":
		if isFloat {
			val = lcg.currentBlock.NewFAdd(leftVal, rightVal)
		} else {
			val = lcg.currentBlock.NewAdd(leftVal, rightVal)
		}
	case "-":
		if isFloat {
			val = lcg.currentBlock.NewFSub(leftVal, rightVal)
		} else {
			val = lcg.currentBlock.NewSub(leftVal, rightVal)
		}
	case "*":
		if isFloat {
			val = lcg.currentBlock.NewFMul(leftVal, rightVal)
		} else {
			val = lcg.currentBlock.NewMul(leftVal, rightVal)
		}
	case "/":
		if isFloat {
			val = lcg.currentBlock.NewFDiv(leftVal, rightVal)
		} else if isUnsigned {
			val = lcg.currentBlock.NewUDiv(leftVal, rightVal)
		} else {
			val = lcg.currentBlock.NewSDiv(leftVal, rightVal)
		}
	case "%":
		if isFloat {
			val = lcg.currentBlock.NewFRem(leftVal, rightVal)
		} else if isUnsigned {
			val = lcg.currentBlock.NewURem(leftVal, rightVal)
		} else {
			val = lcg.currentBlock.NewSRem(leftVal, rightVal)
		}
	case "&":
		val = lcg.currentBlock.NewAnd(leftVal, rightVal)
	case "|":
		val = lcg.currentBlock.NewOr(leftVal, rightVal)
	case "^":
		val = lcg.currentBlock.NewXor(leftVal, rightVal)
	case "<<":
		val = lcg.currentBlock.NewShl(leftVal, rightVal)
	case ">>":
		// Logical shift for unsigned operands, arithmetic for signed.
		if isUnsigned {
			val = lcg.currentBlock.NewLShr(leftVal, rightVal)
		} else {
			val = lcg.currentBlock.NewAShr(leftVal, rightVal)
		}
	}

	if val == nil {
		lcg.errorf(n, "unsupported binary operator %q", n.Operator.Text)
		return
	}

	lcg.values[n] = val
}

func (lcg *LLVMCodeGen) visitFunctionDeclaration(n *ast.FunctionDeclaration) {
	// Generate function name
	var name string
	if n.Signature.Identifier != nil {
		name = n.Signature.Identifier.Text
	} else if n.Signature.Operator != nil {
		// This case should ideally be handled within method mangling for operators
		// or as a global operator function if that's supported.
		// For now, fallback to a generic operator name.
		name = "op_" + n.Signature.Operator.Text
	} else {
		// Anonymous function — generate unique name
		name = fmt.Sprintf("__anon_fn_%d", lcg.anonFnCounter)
		lcg.anonFnCounter++
	}

	// Check if this is a method (inside a struct)
	if lcg.currentStruct != nil {
		// Method: mangle with struct name
		if n.Signature.Operator != nil {
			// Operator overload
			name = lcg.currentStructName + "_op_" + n.Signature.Operator.Text
		} else {
			name = lcg.currentStructName + "_" + name
		}
	} else if lcg.isExported(n) && lcg.moduleName != "" && lcg.moduleName != "main" {
		// Exported function from a library module: mangle with module name
		originalName := name
		name = lcg.moduleName + "__" + name
		// Defer registering under original name so same-module calls resolve
		defer func() {
			if fn, ok := lcg.functions[name]; ok {
				lcg.functions[originalName] = fn
			}
		}()
	}

	isAnonymous := n.Signature.Identifier == nil && n.Signature.Operator == nil

	// Check if already declared
	if _, ok := lcg.functions[name]; ok {
		return
	}

	// Detect captures for anonymous functions (must happen before switching context)
	var captures []capturedVar
	var envStructType *types.StructType
	if isAnonymous && n.Block != nil {
		lambdaArgNames := make(map[string]bool)
		for _, arg := range n.Signature.Arguments {
			if arg.Name != nil {
				lambdaArgNames[arg.Name.Text] = true
			}
		}
		captures = lcg.detectCaptures(n.Block, lambdaArgNames)
		if len(captures) > 0 {
			var envFields []types.Type
			for _, cap := range captures {
				envFields = append(envFields, cap.llvmType)
			}
			envStructType = types.NewStruct(envFields...)
		}
	}

	var params []*ir.Param

	// Add env parameter for anonymous functions (closure calling convention)
	if isAnonymous {
		params = append(params, ir.NewParam("__env", types.I8Ptr))
	}

	// Add 'this' parameter for methods
	if lcg.currentStruct != nil {
		thisParam := ir.NewParam("this", types.NewPointer(lcg.currentStruct))
		params = append(params, thisParam)
	}

	// Get semantic function type
	var funcSemType *ortypes.SignatureType

	var lookupNode ast.Node = n
	if n.Signature.Identifier != nil {
		lookupNode = n.Signature.Identifier
	}

	if nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[lookupNode]; nodeInfo != nil {
		if ft, ok := nodeInfo.Type.(*ortypes.SignatureType); ok {
			funcSemType = ft
		}
	}

	// If still not found, try the other one (if we tried identifier, try n; if we tried n, well n is all we have)
	if funcSemType == nil && n.Signature.Identifier != nil {
		if nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n]; nodeInfo != nil {
			if ft, ok := nodeInfo.Type.(*ortypes.SignatureType); ok {
				funcSemType = ft
			}
		}
	}

	isVariadic := false
	for i, arg := range n.Signature.Arguments {
		if arg.Variadic {
			isVariadic = true
			break // Variadic must be last, don't add to params
		}

		var paramType types.Type
		if funcSemType != nil && i < len(funcSemType.ArgumentTypes) {
			paramType = lcg.getLLVMParamType(funcSemType.ArgumentTypes[i])
		} else {
			paramType = lcg.getLLVMType(arg.Type)
		}

		param := ir.NewParam(arg.Name.Text, paramType)
		params = append(params, param)
		lcg.values[arg.Name] = param
	}

	var returnType types.Type
	if name == "main" {
		// main should always return i32 for compatibility
		returnType = types.I32
	} else if funcSemType != nil {
		returnType = lcg.getLLVMReturnType(funcSemType.ReturnType)
	} else if n.Signature.ReturnType != nil {
		returnType = lcg.getLLVMType(n.Signature.ReturnType)
	} else {
		returnType = types.Void
	}
	fn := lcg.module.NewFunc(name, returnType, params...)
	if isVariadic {
		fn.Sig.Variadic = true
	}
	lcg.functions[name] = fn

	if n.Signature.Extern {
		return
	}

	// Save outer function context (for anonymous functions nested inside another function)
	savedFunc := lcg.currentFunc
	savedBlock := lcg.currentBlock

	block := fn.NewBlock("")

	lcg.currentFunc = fn
	lcg.currentBlock = block

	// Initialize GC at the start of main
	if name == "main" {
		gcInit := lcg.getOrDeclareGCInit()
		block.NewCall(gcInit)
	}

	// Alloca parameters so they are mutable/addressable
	for i, param := range params {
		// Skip env param for anonymous functions (used directly, not via alloca)
		if isAnonymous && i == 0 {
			continue
		}

		// Skip 'this' for methods
		thisIdx := 0
		if isAnonymous {
			thisIdx = 1
		}
		if lcg.currentStruct != nil && i == thisIdx {
			alloca := block.NewAlloca(param.Type())
			block.NewStore(param, alloca)
			continue
		}

		alloca := block.NewAlloca(param.Type())
		block.NewStore(param, alloca)

		// Map to AST argument name
		argIdx := i
		if isAnonymous {
			argIdx-- // skip env
		}
		if lcg.currentStruct != nil {
			argIdx-- // skip 'this'
		}
		argName := n.Signature.Arguments[argIdx].Name
		lcg.values[argName] = alloca
	}

	// Set up capture mappings from env struct (overrides outer variable bindings)
	var savedCaptureMappings map[ast.Node]value.Value
	if isAnonymous && len(captures) > 0 {
		savedCaptureMappings = make(map[ast.Node]value.Value)
		for _, cap := range captures {
			savedCaptureMappings[cap.defineIdent] = lcg.values[cap.defineIdent]
		}

		envParam := params[0] // env i8* param
		envTyped := lcg.currentBlock.NewBitCast(envParam, types.NewPointer(envStructType))

		for i, cap := range captures {
			fieldPtr := lcg.currentBlock.NewGetElementPtr(envStructType, envTyped,
				constant.NewInt(types.I32, 0), constant.NewInt(types.I32, int64(i)))
			lcg.values[cap.defineIdent] = fieldPtr
		}
	}

	// Save and reset deferred calls for this function scope
	savedDefers := lcg.deferredCalls
	lcg.deferredCalls = nil

	ast.Walk(lcg, n.Block)

	// Add implicit return 0 if missing (for main)
	if name == "main" && (len(n.Block.Body) == 0 || !lcg.isTerminator(lcg.currentBlock.Term)) {
		lcg.emitDeferredCalls()
		lcg.currentBlock.NewRet(constant.NewInt(types.I32, 0))
	} else if !lcg.isTerminator(lcg.currentBlock.Term) {
		// Add implicit return void if missing
		if fn.Sig.RetType.Equal(types.Void) {
			lcg.emitDeferredCalls()
			lcg.currentBlock.NewRet(nil)
		} else {
			// A value-returning function whose final block has no terminator.
			// This is either an unreachable merge block (all paths returned)
			// or a genuine missing return; terminate with unreachable so the
			// IR stays valid either way.
			lcg.currentBlock.NewUnreachable()
		}
	}

	// Restore deferred calls from outer function scope
	lcg.deferredCalls = savedDefers

	// Restore capture mappings so outer function sees its own variables again
	if savedCaptureMappings != nil {
		for ident, val := range savedCaptureMappings {
			lcg.values[ident] = val
		}
	}

	// Restore outer function context and set closure value
	if savedFunc != nil {
		lcg.currentFunc = savedFunc
		lcg.currentBlock = savedBlock

		if isAnonymous {
			if len(captures) > 0 {
				// Allocate env struct on the heap and populate with captured values
				mallocFn := lcg.getOrDeclareGCMalloc()

				// Compute sizeof(envStruct) via GEP trick
				nullPtr := constant.NewNull(types.NewPointer(envStructType))
				sizeGEP := lcg.currentBlock.NewGetElementPtr(envStructType, nullPtr, constant.NewInt(types.I32, 1))
				envSize := lcg.currentBlock.NewPtrToInt(sizeGEP, types.I64)

				envRaw := lcg.currentBlock.NewCall(mallocFn, envSize)
				envTyped := lcg.currentBlock.NewBitCast(envRaw, types.NewPointer(envStructType))

				// Store captured values from outer allocas into env struct
				for i, cap := range captures {
					capturedVal := lcg.currentBlock.NewLoad(cap.llvmType, cap.outerValue)
					fieldPtr := lcg.currentBlock.NewGetElementPtr(envStructType, envTyped,
						constant.NewInt(types.I32, 0), constant.NewInt(types.I32, int64(i)))
					lcg.currentBlock.NewStore(capturedVal, fieldPtr)
				}

				lcg.values[n] = lcg.createClosureValue(fn, envRaw)
			} else {
				// No captures — null env
				lcg.values[n] = lcg.createClosureValue(fn, nil)
			}
		}
	}
}

func (lcg *LLVMCodeGen) visitFunctionCall(n *ast.FunctionCall) {
	var name string
	var args []value.Value

	// Check if this is a builtin function call
	if ident, ok := n.Callee.(*ast.Identifier); ok {
		if ident.Text == "len" {
			// Handle len()
			if len(n.Arguments) != 1 {
				return
			}
			arg := n.Arguments[0]
			ast.Walk(lcg, arg.Expression)
			val := lcg.values[arg.Expression]
			if val == nil {
				return
			}

			// Get argument type
			valType := val.Type()

			if structType, ok := valType.(*types.StructType); ok {
				// Slice struct { data*, len }
				// Extract length at index 1
				// Check if it has 2 fields and second is i32
				if len(structType.Fields) == 2 && structType.Fields[1].Equal(types.I32) {
					lenVal := lcg.currentBlock.NewExtractValue(val, 1)
					lcg.values[n] = lenVal
					return
				}
			} else if ptrType, ok := valType.(*types.PointerType); ok {
				if arrayType, ok := ptrType.ElemType.(*types.ArrayType); ok {
					// Fixed array pointer [N x T]*
					length := int64(arrayType.Len)
					lcg.values[n] = constant.NewInt(types.I32, length)
					return
				} else if ptrType.ElemType.Equal(types.I8) {
					// String (i8*): call strlen (returns size_t/i64) and
					// truncate to orlang's int32 len type.
					call := lcg.currentBlock.NewCall(lcg.getOrDeclareStrlen(), val)
					lcg.values[n] = lcg.currentBlock.NewTrunc(call, types.I32)
					return
				}
			}

			// Fallback or error
			return
		}

		if ident.Text == "append" {

			// Handle append(slice, elements...)
			if len(n.Arguments) < 2 {
				return
			}

			// First argument is the slice
			sliceArg := n.Arguments[0]
			ast.Walk(lcg, sliceArg.Expression)
			sliceVal := lcg.values[sliceArg.Expression]
			if sliceVal == nil {
				return
			}

			// Check if it's a slice struct or fixed array pointer
			var elemType types.Type
			var oldDataPtr, oldLen value.Value

			valType := sliceVal.Type()

			// Check semantic type first to handle fixed arrays (which are decayed to pointers)
			nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[sliceArg.Expression]
			var isFixedArray bool
			if nodeInfo != nil {
				if arrayType, ok := nodeInfo.Type.(*ortypes.ArrayType); ok && arrayType.Length >= 0 {
					isFixedArray = true
					elemType = lcg.getLLVMTypeFromSemantic(arrayType.Type)
					oldLen = constant.NewInt(types.I32, int64(arrayType.Length))
					oldDataPtr = sliceVal // Already decayed to T*
				}
			}

			if isFixedArray {
				// Handled above
			} else if structType, ok := valType.(*types.StructType); ok {
				// Slice struct { data*, len }
				if len(structType.Fields) != 2 {
					return
				}

				// Extract element type from slice data pointer
				dataPtrType := structType.Fields[0]
				elemPtrType, ok := dataPtrType.(*types.PointerType)
				if !ok {
					return
				}
				elemType = elemPtrType.ElemType

				// Extract old data pointer and length
				oldDataPtr = lcg.currentBlock.NewExtractValue(sliceVal, 0)
				oldLen = lcg.currentBlock.NewExtractValue(sliceVal, 1)

			} else {
				return
			}

			// Calculate new length
			numNewElems := len(n.Arguments) - 1
			newLenVal := lcg.currentBlock.NewAdd(oldLen, constant.NewInt(types.I32, int64(numNewElems)))

			// Allocate new array: GC_malloc(newLen * sizeof(elem))
			elemSize := lcg.getSizeOf(elemType)
			newLenI64 := lcg.currentBlock.NewSExt(newLenVal, types.I64)
			totalSize := lcg.currentBlock.NewMul(newLenI64, constant.NewInt(types.I64, elemSize))

			// Declare GC_malloc if not exists
			mallocFn := lcg.getOrDeclareGCMalloc()

			newDataI8 := lcg.currentBlock.NewCall(mallocFn, totalSize)
			newDataPtr := lcg.currentBlock.NewBitCast(newDataI8, types.NewPointer(elemType))

			// Copy old elements using memcpy
			oldLenI64 := lcg.currentBlock.NewSExt(oldLen, types.I64)
			oldSize := lcg.currentBlock.NewMul(oldLenI64, constant.NewInt(types.I64, elemSize))

			memcpyFn := lcg.getOrDeclareMemcpy()

			oldDataI8 := lcg.currentBlock.NewBitCast(oldDataPtr, types.I8Ptr)
			lcg.currentBlock.NewCall(memcpyFn, newDataI8, oldDataI8, oldSize)

			// Append new elements
			for i, arg := range n.Arguments[1:] {
				ast.Walk(lcg, arg.Expression)
				elemVal := lcg.values[arg.Expression]
				if elemVal == nil {
					continue
				}

				// Calculate index: oldLen + i
				idx := lcg.currentBlock.NewAdd(oldLen, constant.NewInt(types.I32, int64(i)))

				// Get pointer to element: newDataPtr[idx]
				elemPtr := lcg.currentBlock.NewGetElementPtr(elemType, newDataPtr, idx)

				// Store element
				lcg.currentBlock.NewStore(elemVal, elemPtr)
			}

			// Create new slice struct
			appendSliceType := types.NewStruct(types.NewPointer(elemType), types.I32)
			var newSlice value.Value = constant.NewStruct(
				appendSliceType,
				constant.NewNull(types.NewPointer(elemType)),
				constant.NewInt(types.I32, 0),
			)
			newSlice = lcg.currentBlock.NewInsertValue(newSlice, newDataPtr, 0)
			newSlice = lcg.currentBlock.NewInsertValue(newSlice, newLenVal, 1)

			lcg.values[n] = newSlice
			return
		}

		if ident.Text == "str" {
			// Handle str(number) -> string conversion
			if len(n.Arguments) != 1 {
				return
			}
			arg := n.Arguments[0]
			ast.Walk(lcg, arg.Expression)
			val := lcg.values[arg.Expression]
			if val == nil {
				return
			}

			// Pick a printf format matching the value's actual type; C
			// varargs promote float to double, so extend floats manually.
			format := "%d"
			argSem := lcg.semanticType(arg.Expression)
			unsigned := isUnsignedType(argSem)
			switch t := val.Type().(type) {
			case *types.FloatType:
				format = "%g"
				if t.Kind == types.FloatKindFloat {
					val = lcg.currentBlock.NewFPExt(val, types.Double)
				}
			case *types.IntType:
				switch {
				case t.BitSize == 64 && unsigned:
					format = "%llu"
				case t.BitSize == 64:
					format = "%lld"
				case unsigned:
					if t.BitSize < 32 {
						val = lcg.currentBlock.NewZExt(val, types.I32)
					}
					format = "%u"
				case t.BitSize == 1:
					val = lcg.currentBlock.NewZExt(val, types.I32)
				case t.BitSize < 32:
					val = lcg.currentBlock.NewSExt(val, types.I32)
				}
			}

			// 32 bytes covers any 64-bit integer and %g float rendering.
			mallocFn := lcg.getOrDeclareGCMalloc()
			bufSize := constant.NewInt(types.I64, 32)
			buf := lcg.currentBlock.NewCall(mallocFn, bufSize)

			// Declare snprintf if not exists
			var snprintfFn *ir.Func
			if fn, ok := lcg.functions["snprintf"]; ok {
				snprintfFn = fn
			} else {
				snprintfFn = lcg.module.NewFunc("snprintf", types.I32,
					ir.NewParam("buf", types.I8Ptr),
					ir.NewParam("size", types.I64),
					ir.NewParam("fmt", types.I8Ptr))
				snprintfFn.Sig.Variadic = true
				lcg.functions["snprintf"] = snprintfFn
			}

			fmtStr := lcg.addStringConstant(format)
			lcg.currentBlock.NewCall(snprintfFn, buf, bufSize, fmtStr, val)

			lcg.values[n] = buf
			return
		}
	}

	// Check if this is a type cast (e.g., int64(x), int32(y))
	if ident, ok := n.Callee.(*ast.Identifier); ok {
		name = ident.Text

		// Check if it's a method call on 'this' implicitly?
		// Or just a global function.
		// If we are in a method, and 'name' is a method of current struct, we should treat it as this.name()
		// But for now let's assume explicit this.method() or global function.
		// Check if it's a method call
	} else if member, ok := n.Callee.(*ast.MemberExpression); ok {
		// ... existing method resolution logic ...
		// We need to check if target is an interface

		targetAddr := lcg.getAddress(member.Target)
		if targetAddr != nil {
			// Check if target is interface
			// We need semantic type of target
			var targetTyp ortypes.Type
			nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[member.Target]
			if nodeInfo != nil {
				targetTyp = nodeInfo.Type
			}

			if ifaceTyp, ok := targetTyp.(*ortypes.InterfaceType); ok {
				// Interface method call
				// 1. Load interface value (it's a struct { i8*, i8* })
				// targetAddr is pointer to interface struct

				// We need to load the interface struct
				// Actually, targetAddr is the address of the variable holding the interface.
				// So we load the interface value.
				ifaceVal := lcg.currentBlock.NewLoad(lcg.getInterfaceType(), targetAddr)

				// 2. Extract data pointer and itable pointer
				dataPtr := lcg.currentBlock.NewExtractValue(ifaceVal, 0)
				itablePtr := lcg.currentBlock.NewExtractValue(ifaceVal, 1)

				// 3. Find method index in itable
				methodIdx := -1
				for i, fn := range ifaceTyp.Functions {
					if fn.Name == member.Property.Text {
						methodIdx = i
						break
					}
				}

				if methodIdx == -1 {
					// Should not happen
					return
				}

				// 4. Get function pointer from itable
				// itablePtr is i8*. Cast to { i32, [0 x i8*] }*
				itableStructType := types.NewStruct(types.I32, types.NewArray(0, types.I8Ptr))
				itableTyped := lcg.currentBlock.NewBitCast(itablePtr, types.NewPointer(itableStructType))

				// Get pointer to the function pointers array (index 1)
				// GEP(itableTyped, 0, 1) -> pointer to [0 x i8*]
				arrayPtr := lcg.currentBlock.NewGetElementPtr(itableStructType, itableTyped,
					constant.NewInt(types.I32, 0),
					constant.NewInt(types.I32, 1))

				// Get pointer to method slot
				// GEP(arrayPtr, 0, methodIdx)
				methodSlot := lcg.currentBlock.NewGetElementPtr(types.NewArray(0, types.I8Ptr), arrayPtr,
					constant.NewInt(types.I32, 0),
					constant.NewInt(types.I32, int64(methodIdx)))

				// Load function pointer (i8*)
				fnPtrRaw := lcg.currentBlock.NewLoad(types.I8Ptr, methodSlot)

				// 5. Cast function pointer to correct type
				// We need the signature of the interface method
				// But with 'this' as i8*
				methodSig := ifaceTyp.Functions[methodIdx].Type

				var paramTypes []types.Type
				paramTypes = append(paramTypes, types.I8Ptr) // 'this'
				for _, arg := range methodSig.ArgumentTypes {
					paramTypes = append(paramTypes, lcg.getLLVMTypeFromSemantic(arg))
				}

				returnType := lcg.getLLVMTypeFromSemantic(methodSig.ReturnType)
				fnType := types.NewPointer(types.NewFunc(returnType, paramTypes...))

				fnPtr := lcg.currentBlock.NewBitCast(fnPtrRaw, fnType)

				// 6. Prepare arguments
				var callArgs []value.Value
				callArgs = append(callArgs, dataPtr)

				for i, arg := range n.Arguments {
					ast.Walk(lcg, arg)
					val := lcg.values[arg.Expression]

					// Cast argument if needed
					// We need expected type from signature
					expectedTyp := methodSig.ArgumentTypes[i]

					// Get actual type
					var actualTyp ortypes.Type
					nodeInfoArg := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[arg.Expression]
					if nodeInfoArg != nil {
						actualTyp = nodeInfoArg.Type
					}

					val = lcg.castIfNeeded(val, actualTyp, expectedTyp)
					callArgs = append(callArgs, val)
				}

				// 7. Call
				if fnPtr == nil {
				}

				call := lcg.currentBlock.NewCall(fnPtr, callArgs...)
				lcg.values[n] = call
				return
			}
		}

		// ... existing struct method logic ...
		// Method call
		// Resolve target
		targetAddr = lcg.getAddress(member.Target)
		if targetAddr != nil {
			// Get struct type
			if ptrType, ok := targetAddr.Type().(*types.PointerType); ok {
				// Check for double indirection (pointer to pointer to struct)
				if elemPtrType, ok := ptrType.ElemType.(*types.PointerType); ok {
					// Load the pointer
					targetAddr = lcg.currentBlock.NewLoad(elemPtrType, targetAddr)
					ptrType = elemPtrType
				}

				structName := ptrType.ElemType.Name()

				if structName != "" {
					methodName := member.Property.Text
					mangledName := structName + "_" + methodName

					// Check if function exists
					if _, ok := lcg.functions[mangledName]; ok {
						name = mangledName
						args = append(args, targetAddr) // Pass 'this'
					} else {
						// Check if it is a valid method in the struct type
						nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[member.Target]
						if nodeInfo != nil {
							if structTyp, ok := nodeInfo.Type.(*ortypes.StructType); ok {
								if has, methodTyp := structTyp.HasFunction(methodName); has {
									if sig, ok := methodTyp.(*ortypes.SignatureType); ok {
										// Declare it
										var params []*ir.Param
										// Add 'this' param
										params = append(params, ir.NewParam("this", types.NewPointer(lcg.structs[structTyp.Name])))

										for i, argType := range sig.ArgumentTypes {
											params = append(params, ir.NewParam(fmt.Sprintf("arg%d", i), lcg.getLLVMParamType(argType)))
										}

										returnType := lcg.getLLVMReturnType(sig.ReturnType)
										fn := lcg.module.NewFunc(mangledName, returnType, params...)
										lcg.functions[mangledName] = fn

										name = mangledName
										args = append(args, targetAddr)
									}
								}
							}
						}
					}
				}
			}
		}
		if name == "" {
			// Fallback if not resolved as method (e.g. function pointer in struct field?)
			// For now just return
			return
		}
	} else {
		// TODO: Handle other callees
		return
	}

	// Check if it's a type cast
	if ident, ok := n.Callee.(*ast.Identifier); ok {
		// Check for primitive types
		isCast := false
		switch ident.Text {
		case "float32", "float64", "int64", "int32", "int16", "int8", "uint64", "uint32", "uint16", "uint8":
			isCast = true
		}

		if isCast {
			// It's a cast
			if len(n.Arguments) != 1 {
				lcg.errorf(n, "type cast %s() must have exactly one argument", ident.Text)
				return
			}
			arg := n.Arguments[0]
			ast.Walk(lcg, arg)
			val := lcg.values[arg.Expression]

			// Perform cast
			// We need target type
			var targetType types.Type
			switch ident.Text {
			case "float32":
				targetType = types.Float
			case "float64":
				targetType = types.Double
			case "int64", "uint64":
				targetType = types.I64
			case "int32", "uint32":
				targetType = types.I32
			case "int16", "uint16":
				targetType = types.I16
			case "int8", "uint8":
				targetType = types.I8
			}

			// Cast val to targetType
			targetIsSigned := true
			switch ident.Text {
			case "uint8", "uint16", "uint32", "uint64":
				targetIsSigned = false
			}

			sourceIsSigned := true
			nodeInfoArg := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[arg.Expression]
			if nodeInfoArg != nil && nodeInfoArg.Type != nil {
				typeName := nodeInfoArg.Type.GetName()
				if strings.HasPrefix(typeName, "uint") {
					sourceIsSigned = false
				}
			}

			castVal := lcg.castValue(val, targetType, sourceIsSigned, targetIsSigned)
			lcg.values[n] = castVal
			return
		}
	}

	fn, ok := lcg.functions[name]
	if !ok {
		// Not a direct function — check if it's a function-typed variable/parameter (indirect call via closure struct)
		if ident, ok := n.Callee.(*ast.Identifier); ok {
			ast.Walk(lcg, ident)
			closureVal := lcg.values[ident]
			if closureVal != nil {
				nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[ident]
				if nodeInfo != nil {
					if sig, ok := nodeInfo.Type.(*ortypes.SignatureType); ok {
						// Extract fn_ptr and env_ptr from closure struct { i8*, i8* }
						fnPtrRaw := lcg.currentBlock.NewExtractValue(closureVal, 0)
						envPtr := lcg.currentBlock.NewExtractValue(closureVal, 1)

						// Cast fn_ptr to the correct function type (with env as first param)
						closureFuncType := lcg.getClosureFuncType(sig)
						fnPtr := lcg.currentBlock.NewBitCast(fnPtrRaw, types.NewPointer(closureFuncType))

						// Build args: env first, then user args
						var callArgs []value.Value
						callArgs = append(callArgs, envPtr)
						for _, arg := range n.Arguments {
							ast.Walk(lcg, arg)
							argVal := lcg.values[arg.Expression]
							if argVal != nil {
								callArgs = append(callArgs, argVal)
							}
						}

						result := lcg.currentBlock.NewCall(fnPtr, callArgs...)
						lcg.values[n] = result
						return
					}
				}
			}
		}
		lcg.errorf(n, "undefined function: %s", name)
		return
	}

	if fn == nil {
		lcg.errorf(n, "internal error: function %s resolved to nil", name)
		return
	}

	// Resolve function signature to handle named arguments and type casting
	var signature *ortypes.SignatureType
	if ident, ok := n.Callee.(*ast.Identifier); ok {
		nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[ident]
		if nodeInfo != nil {
			signature, _ = nodeInfo.Type.(*ortypes.SignatureType)
		}
	} else if member, ok := n.Callee.(*ast.MemberExpression); ok {
		// Method call resolution
		nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[member.Target]
		if nodeInfo != nil && nodeInfo.Type != nil {
			targetObjTyp := nodeInfo.Type
			if typeWithMethods, ok := targetObjTyp.(ortypes.TypeWithMethods); ok {
				if has, typ := typeWithMethods.HasFunction(member.Property.Text); has {
					signature, _ = typ.(*ortypes.SignatureType)
				}
			}
		}
	}

	// Use ArgumentMatcher to resolve arguments
	matcher := NewArgumentMatcher(lcg, signature, fn, args)
	finalArgs, err := matcher.Match(n.Arguments)
	if err != nil {
		lcg.errorf(n, "call to %s: %s", name, err)
		return
	}

	val := lcg.currentBlock.NewCall(fn, finalArgs...)
	lcg.values[n] = val
}

func (lcg *LLVMCodeGen) visitIdentifier(n *ast.Identifier) {
	if n == nil {
		return
	}
	nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n]
	if nodeInfo == nil {
		return
	}

	details := nodeInfo.Scope.GetDetails(n.Text, true)
	if details == nil {
		return
	}

	if val, ok := lcg.values[details.DefineIdentifier]; ok {
		// Check if we have metadata for this alloca
		if metadata, ok := lcg.allocaMetadata[val]; ok {
			// Use type category to determine how to handle the value
			switch metadata.Category {
			case StructRefType:
				// Structs stay as pointers (reference semantics)
				lcg.values[n] = val
				return
			case ArrayDecayType:
				// Arrays decay to pointer to first element
				lcg.values[n] = lcg.arrayHelper.DecayArray(val)
				return
			case SliceValueType, InterfaceValueType, TupleValueType, PrimitiveType:
				// These should be loaded (value semantics)
				if ptrType, isPtr := val.Type().(*types.PointerType); isPtr {
					_, isInst := val.(ir.Instruction)
					_, isGlobal := val.(*ir.Global)
					if isInst || isGlobal {
						load := lcg.currentBlock.NewLoad(ptrType.ElemType, val)
						lcg.values[n] = load
						return
					}
				}
			}
		} else {
			// No metadata - fallback to old behavior
			// If it's an alloca or global (pointer), load it
			if ptrType, isPtr := val.Type().(*types.PointerType); isPtr {
				_, isInst := val.(ir.Instruction)
				_, isGlobal := val.(*ir.Global)
				if isInst || isGlobal {
					// Determine category from types
					category := GetTypeCategory(nodeInfo.Type, ptrType.ElemType)

					switch category {
					case StructRefType:
						// Keep as pointer
						lcg.values[n] = val
						return
					case ArrayDecayType:
						// Decay array
						lcg.values[n] = lcg.arrayHelper.DecayArray(val)
						return
					default:
						// Load value
						load := lcg.currentBlock.NewLoad(ptrType.ElemType, val)
						lcg.values[n] = load
						return
					}
				}
			}
		}

		lcg.values[n] = val
		return
	}

	// Check if it's a function name used as a value — create closure struct
	if fn, ok := lcg.functions[n.Text]; ok {
		wrapper := lcg.getOrCreateWrapper(n.Text, fn)
		lcg.values[n] = lcg.createClosureValue(wrapper, nil)
	}
}

func (lcg *LLVMCodeGen) visitDeferStatement(n *ast.DeferStatement) {
	lcg.deferredCalls = append(lcg.deferredCalls, n.Call)
}

func (lcg *LLVMCodeGen) emitDeferredCalls() {
	for i := len(lcg.deferredCalls) - 1; i >= 0; i-- {
		ast.Walk(lcg, lcg.deferredCalls[i])
	}
}

func (lcg *LLVMCodeGen) visitReturnStatement(n *ast.ReturnStatement) {
	if n.Expression != nil {
		ast.Walk(lcg, n.Expression)
		val := lcg.values[n.Expression]

		// Check if we need to load the value (e.g. returning struct value from pointer)
		// We need to know the expected return type of the function
		returnType := lcg.currentFunc.Sig.RetType

		// If return type is not a pointer, but val is a pointer, we might need to load it
		// This happens for structs which are now returned by value
		if _, ok := returnType.(*types.PointerType); !ok {
			if ptr, ok := val.Type().(*types.PointerType); ok {
				// Check if it's a pointer to the return type
				if ptr.ElemType.Equal(returnType) {
					val = lcg.currentBlock.NewLoad(returnType, val)
				}
			}
		}

		// Emit deferred calls before returning
		lcg.emitDeferredCalls()
		lcg.currentBlock.NewRet(val)
	} else {
		lcg.emitDeferredCalls()
		lcg.currentBlock.NewRet(nil)
	}
}

func (lcg *LLVMCodeGen) visitValueExpression(n *ast.ValueExpression) {
	var val value.Value

	if n.Token.Type == scanner.TokenTypeString {
		strVal := n.Token.Value.(string)
		val = lcg.addStringConstant(strVal)
	} else if n.Token.Value != nil {
		if i, ok := n.Token.Value.(int64); ok {
			// Check semantic type to determine int width
			intType := types.I32
			if i > math.MaxInt32 || i < math.MinInt32 {
				intType = types.I64
			}
			if nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[n]; nodeInfo != nil && nodeInfo.Type != nil {
				if resolved, ok := lcg.getLLVMTypeFromSemantic(nodeInfo.Type).(*types.IntType); ok {
					intType = resolved
				}
			}
			val = constant.NewInt(intType, i)
		} else if b, ok := n.Token.Value.(bool); ok {
			intVal := int64(0)
			if b {
				intVal = 1
			}
			val = constant.NewInt(types.I1, intVal)
		} else if f, ok := n.Token.Value.(float64); ok {
			// Emit the constant at the width the analyser resolved for this
			// literal (float32 by default, float64 when required).
			floatType := types.Float
			if f > math.MaxFloat32 || f < -math.MaxFloat32 {
				floatType = types.Double
			}
			if resolved, ok := lcg.getLLVMTypeFromSemantic(lcg.semanticType(n)).(*types.FloatType); ok {
				floatType = resolved
			}
			val = constant.NewFloat(floatType, f)
		}
	}

	lcg.values[n] = val
}

func (lcg *LLVMCodeGen) addStringConstant(str string) value.Value {
	// Add null terminator
	str += "\x00"
	c := constant.NewCharArrayFromString(str)
	// Use module-specific name for the global to avoid conflicts
	globalName := ""
	if lcg.moduleName != "" && lcg.moduleName != "main" {
		globalName = lcg.moduleName + "_str"
	} else {
		globalName = "str"
	}

	globalName = fmt.Sprintf("%s_%d", globalName, lcg.stringCount)
	lcg.stringCount++

	g := lcg.module.NewGlobalDef(globalName, c)
	g.Immutable = true

	// Get pointer to first element
	zero := constant.NewInt(types.I32, 0)
	return constant.NewGetElementPtr(c.Type(), g, zero, zero)
}

func (lcg *LLVMCodeGen) visitStringConcat(n ast.Node, left, right value.Value) {
	strlenFn := lcg.getOrDeclareStrlen()
	memcpyFn := lcg.getOrDeclareMemcpy()

	// 1. Get lengths (size_t / i64)
	len1 := lcg.currentBlock.NewCall(strlenFn, left)
	len2 := lcg.currentBlock.NewCall(strlenFn, right)

	// 2. Calculate total length + 1 for null terminator
	totalLen := lcg.currentBlock.NewAdd(len1, len2)
	totalLenPlus1 := lcg.currentBlock.NewAdd(totalLen, constant.NewInt(types.I64, 1))

	// 3. Allocate new buffer via GC
	mallocFn := lcg.getOrDeclareGCMalloc()
	newBuf := lcg.currentBlock.NewCall(mallocFn, totalLenPlus1)

	// 4. Copy first string
	lcg.currentBlock.NewCall(memcpyFn, newBuf, left, len1)

	// 5. Copy second string (including null terminator)
	offset := lcg.currentBlock.NewGetElementPtr(types.I8, newBuf, len1)
	len2Plus1 := lcg.currentBlock.NewAdd(len2, constant.NewInt(types.I64, 1))
	lcg.currentBlock.NewCall(memcpyFn, offset, right, len2Plus1)

	lcg.values[n] = newBuf
}

func (lcg *LLVMCodeGen) isTerminator(term ir.Terminator) bool {
	return term != nil
}

func (lcg *LLVMCodeGen) visitIfStatement(n *ast.IfStatement) {
	// Generate condition
	ast.Walk(lcg, n.Condition)
	condVal := lcg.values[n.Condition]

	// Create blocks
	thenBlock := lcg.currentFunc.NewBlock("")
	mergeBlock := lcg.currentFunc.NewBlock("")

	var elseBlock *ir.Block
	if n.Else != nil {
		elseBlock = lcg.currentFunc.NewBlock("")
		lcg.currentBlock.NewCondBr(condVal, thenBlock, elseBlock)
	} else {
		lcg.currentBlock.NewCondBr(condVal, thenBlock, mergeBlock)
	}

	// Generate Then block
	lcg.currentBlock = thenBlock
	ast.Walk(lcg, n.Block)
	if !lcg.isTerminator(lcg.currentBlock.Term) {
		lcg.currentBlock.NewBr(mergeBlock)
	}

	// Generate Else block if exists
	if n.Else != nil {
		lcg.currentBlock = elseBlock
		ast.Walk(lcg, n.Else)
		if !lcg.isTerminator(lcg.currentBlock.Term) {
			lcg.currentBlock.NewBr(mergeBlock)
		}
	}

	// Continue with merge block
	lcg.currentBlock = mergeBlock
}

func (lcg *LLVMCodeGen) visitSwitchStatement(n *ast.SwitchStatement) {
	// Evaluate the switch expression
	ast.Walk(lcg, n.Expression)
	switchVal := lcg.values[n.Expression]

	mergeBlock := lcg.currentFunc.NewBlock("")

	// Find default case index (-1 if none)
	defaultIdx := -1
	for i, c := range n.Cases {
		if c.IsDefault {
			defaultIdx = i
			break
		}
	}

	// Build chain of condition checks and case bodies
	for i, c := range n.Cases {
		if c.IsDefault {
			continue // handle default as the fallthrough target
		}

		// Evaluate case value
		ast.Walk(lcg, c.Value)
		caseVal := lcg.values[c.Value]

		// Compare — use strcmp for strings (i8*)
		var cond value.Value
		isStr := false
		if ptrL, ok := switchVal.Type().(*types.PointerType); ok {
			if ptrR, ok := caseVal.Type().(*types.PointerType); ok {
				if ptrL.ElemType.Equal(types.I8) && ptrR.ElemType.Equal(types.I8) {
					isStr = true
				}
			}
		}
		if isStr {
			var strcmpFn *ir.Func
			if fn, ok := lcg.functions["strcmp"]; ok {
				strcmpFn = fn
			} else {
				strcmpFn = lcg.module.NewFunc("strcmp", types.I32,
					ir.NewParam("s1", types.I8Ptr),
					ir.NewParam("s2", types.I8Ptr))
				lcg.functions["strcmp"] = strcmpFn
			}
			cmpResult := lcg.currentBlock.NewCall(strcmpFn, switchVal, caseVal)
			cond = lcg.currentBlock.NewICmp(enum.IPredEQ, cmpResult, constant.NewInt(types.I32, 0))
		} else if isFloatLLVMType(switchVal.Type()) || isFloatLLVMType(caseVal.Type()) {
			l, r := lcg.unifyNumericOperands(switchVal, caseVal, n.Expression, c.Value)
			cond = lcg.currentBlock.NewFCmp(enum.FPredOEQ, l, r)
		} else {
			l, r := lcg.unifyNumericOperands(switchVal, caseVal, n.Expression, c.Value)
			cond = lcg.currentBlock.NewICmp(enum.IPredEQ, l, r)
		}

		caseBlock := lcg.currentFunc.NewBlock("")
		nextBlock := lcg.currentFunc.NewBlock("")

		lcg.currentBlock.NewCondBr(cond, caseBlock, nextBlock)

		// Generate case body
		lcg.currentBlock = caseBlock
		ast.Walk(lcg, c.Block)
		if !lcg.isTerminator(lcg.currentBlock.Term) {
			lcg.currentBlock.NewBr(mergeBlock)
		}

		// Move to next comparison block
		lcg.currentBlock = nextBlock

		// If this is the last non-default case and there's no default,
		// the nextBlock falls through to merge
		if i == len(n.Cases)-1 || (i == len(n.Cases)-2 && defaultIdx == len(n.Cases)-1) {
			// will be handled below
		}
	}

	// Handle default case or fall through to merge
	if defaultIdx >= 0 {
		ast.Walk(lcg, n.Cases[defaultIdx].Block)
		if !lcg.isTerminator(lcg.currentBlock.Term) {
			lcg.currentBlock.NewBr(mergeBlock)
		}
	} else {
		lcg.currentBlock.NewBr(mergeBlock)
	}

	lcg.currentBlock = mergeBlock
}

func (lcg *LLVMCodeGen) visitStruct(n *ast.Struct) {
	name := n.Name.Text

	// Create struct type
	var fields []types.Type
	fieldIndices := make(map[string]int)
	for i, v := range n.Variables {
		fields = append(fields, lcg.getLLVMType(v.Type))
		fieldIndices[v.Name.Text] = i
	}

	structType := types.NewStruct(fields...)
	typeDef := lcg.module.NewTypeDef(name, structType)
	lcg.structs[name] = typeDef
	lcg.structDefinitions[name] = structType
	lcg.structFields[name] = fieldIndices

	// Visit methods
	lcg.currentStruct = structType
	lcg.currentStructName = name

	for _, fn := range n.Functions {
		lcg.visitFunctionDeclaration(fn)
	}

	lcg.currentStruct = nil
	lcg.currentStructName = ""
}

func (lcg *LLVMCodeGen) visitEnum(n *ast.Enum) {
	name := n.Name.Text
	lcg.enumValues[name] = make(map[string]int32)
	for i, val := range n.Values {
		lcg.enumValues[name][val.Name.Text] = int32(i)
	}
}

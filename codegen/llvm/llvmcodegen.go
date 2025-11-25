package llvm

import (
	"fmt"
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
	}
	// Initialize helpers that need reference to lcg
	lcg.arrayHelper = NewArrayHelper(lcg)
	return lcg
}

func (lcg *LLVMCodeGen) SetModuleName(name string) {
	lcg.moduleName = name
}

func (lcg *LLVMCodeGen) Generate(file *ast.File) string {
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

	// Allocate struct
	alloca := lcg.currentBlock.NewAlloca(structType)

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

func (lcg *LLVMCodeGen) getAddress(n ast.Node) value.Value {
	if ident, ok := n.(*ast.Identifier); ok {
		nodeInfo := lcg.analyserInfo.FileInfo[lcg.currentFile].NodeInfo[ident]
		if nodeInfo != nil {
			details := nodeInfo.Scope.GetDetails(ident.Text, true)
			if details != nil {
				if val, ok := lcg.values[details.DefineIdentifier]; ok {
					return val
				} else {
				}
			} else {
			}
		} else {
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
	case *ast.Struct:
		lcg.visitStruct(n)
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
	case *ast.IndexExpression:
		lcg.visitIndexExpression(n)
		return nil
	case *ast.ForLoop:
		lcg.visitForLoop(n)
		return nil
	case *ast.UnaryExpression:
		lcg.visitUnaryExpression(n)
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

	// TODO: Handle prefix unary expressions
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
	ast.Walk(lcg, n.Block)

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

func (lcg *LLVMCodeGen) visitVariableDeclaration(n *ast.VariableDeclaration) {
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

		// If the value is already an alloca (e.g., for arrays/structs),
		// just use it directly instead of creating a new alloca
		if allocaInst, isAlloca := val.(*ir.InstAlloca); isAlloca {
			lcg.values[n.Name] = val

			// Store metadata for this reused alloca
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
			category := GetTypeCategory(semType, allocaInst.ElemType)

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

func (lcg *LLVMCodeGen) visitAssigment(n *ast.Assigment) {
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
		fmt.Printf("ERROR: Assignment value is nil for left=%T, right=%T\n", n.Left, n.Right)
		return
	}
	if addr == nil {
		fmt.Printf("ERROR: Assignment addr is nil for left=%T\n", n.Left)
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
		}

		if structName != "" {
			fnName := structName + "_op_" + n.Operator.Text
			if fn, ok := lcg.functions[fnName]; ok {
				// Call the overloaded operator
				// Arguments: this (left), left, right
				// Note: The operator function is defined as fn +(left:Point, right:Point) inside struct Point
				// So it has 3 arguments: this, left, right
				val = lcg.currentBlock.NewCall(fn, leftVal, leftVal, rightVal)
				lcg.values[n] = val
				return
			}
		}
	}

	isFloat := false
	if leftVal.Type().Equal(types.Float) || leftVal.Type().Equal(types.Double) {
		isFloat = true
	}

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
		} else {
			val = lcg.currentBlock.NewSDiv(leftVal, rightVal)
		}
	}

	if val == nil {
	} else {
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
		// Default name for functions without identifier (e.g., main)
		name = "main"
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
		name = lcg.moduleName + "__" + name
	}

	// Check if already declared
	if _, ok := lcg.functions[name]; ok {
		return
	}

	var params []*ir.Param

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

	block := fn.NewBlock("")

	lcg.currentFunc = fn
	lcg.currentBlock = block

	// Alloca parameters so they are mutable/addressable
	for i, param := range params {
		// Skip 'this' for now, or handle it
		if lcg.currentStruct != nil && i == 0 {
			alloca := block.NewAlloca(param.Type())
			block.NewStore(param, alloca)
			// We map 'this' manually since it's not in AST arguments
			// We can use a special AST node or string key if we had one
			// But for now, let's just rely on 'this' identifier resolution
			// The analyser should resolve 'this' to a ScopeItem.
			// We need to make sure we can look it up.
			// Actually, 'this' is usually implicit in scope.
			// Let's assume the analyser handles 'this' resolution to a special identifier.
			// If not, we might need to hack it.
			// For now, let's just proceed.
			continue
		}

		alloca := block.NewAlloca(param.Type())
		block.NewStore(param, alloca)

		// Update mapping to point to alloca
		// Adjust index for 'this'
		argIdx := i
		if lcg.currentStruct != nil {
			argIdx--
		}
		argName := n.Signature.Arguments[argIdx].Name
		lcg.values[argName] = alloca
	}

	ast.Walk(lcg, n.Block)

	// Add implicit return 0 if missing (for main)
	if name == "main" && (len(n.Block.Body) == 0 || !lcg.isTerminator(lcg.currentBlock.Term)) {
		lcg.currentBlock.NewRet(constant.NewInt(types.I32, 0))
	} else if !lcg.isTerminator(lcg.currentBlock.Term) {
		// Add implicit return void if missing
		if fn.Sig.RetType.Equal(types.Void) {
			lcg.currentBlock.NewRet(nil)
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
					// String (i8*)
					// Call strlen
					// We need to declare strlen if not exists
					strlenName := "strlen"
					var strlen *ir.Func
					if fn, ok := lcg.functions[strlenName]; ok {
						strlen = fn
					} else {
						strlen = lcg.module.NewFunc(strlenName, types.I32, ir.NewParam("str", types.I8Ptr))
						lcg.functions[strlenName] = strlen
					}
					call := lcg.currentBlock.NewCall(strlen, val)
					lcg.values[n] = call
					return
				}
			}

			// Fallback or error
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
				panic("Type cast must have exactly one argument")
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
		// Function not found - this shouldn't happen if analyzer did its job
		// But we should handle it gracefully
		panic(fmt.Sprintf("undefined function: %s (available: %v)", name, getKeys(lcg.functions)))
	}

	if fn == nil {
		panic(fmt.Sprintf("function %s is nil in lcg.functions", name))
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
		panic(err)
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
					if _, isInst := val.(ir.Instruction); isInst {
						load := lcg.currentBlock.NewLoad(ptrType.ElemType, val)
						lcg.values[n] = load
						return
					}
				}
			}
		} else {
			// No metadata - fallback to old behavior
			// If it's an alloca (pointer), load it
			if ptrType, isPtr := val.Type().(*types.PointerType); isPtr {
				if _, isInst := val.(ir.Instruction); isInst {
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

		lcg.currentBlock.NewRet(val)
	} else {
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
			val = constant.NewInt(types.I32, i)
		} else if b, ok := n.Token.Value.(bool); ok {
			intVal := int64(0)
			if b {
				intVal = 1
			}
			val = constant.NewInt(types.I1, intVal)
		} else if f, ok := n.Token.Value.(float64); ok {
			// Default to float32 unless it's too big?
			// Analyzer logic:
			// if n.Token.Value.(float64) > math.MaxFloat32 { return types.Float64Type }
			// return types.Float32Type

			// For now, let's default to Float (float32) to match analyzer default
			// If it's larger than MaxFloat32, we should use Double.
			// But constant.NewFloat takes float64.

			// We can check semantic info if available?
			// But visitValueExpression doesn't look up semantic info usually.

			// Let's use Float (32-bit) as default.
			val = constant.NewFloat(types.Float, f)
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

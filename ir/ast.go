package ir

// Node represents a node in the IR AST
type Node interface {
	// TODO add common methods if any
}

// Type represents a type in the IR
type Type interface {
	Node
	// TODO add common methods if any
}

// Instruction represents an instruction in the IR
type Instruction interface {
	Node
	// TODO add common methods if any
}

// File represents a parsed IR file
type File struct {
	Nodes []Node
}

// TypeDefinition represents a type definition (e.g., `type Foo {int32, int32}`)
type TypeDefinition struct {
	Name string
	Type Type
}

// StructType represents a struct type (e.g., `{int32, int32}`)
type StructType struct {
	Fields []Type
}

// PointerType represents a pointer type (e.g., `ptr<Foo>`)
type PointerType struct {
	ElementType Type
}

// BasicType represents a basic type (e.g., `int32`, `float64`, `bool`)
type BasicType struct {
	Name string
}

// FunctionDefinition represents a function definition
type FunctionDefinition struct {
	Name       string
	Parameters []*VarDeclaration
	ReturnType Type
	Body       []Instruction
	Labels     map[string]int // Maps label name to instruction index in Body
}

// VarDeclaration represents a variable declaration (e.g., `%x : int32`)
type VarDeclaration struct {
	Name string
	Type Type
}

// CallInstruction represents a call instruction (e.g., `call foobar(%x, %y) : ptr<Foo>`)
type CallInstruction struct {
	VarName    string // Name of the variable to store the result
	FunctionName string
	Arguments  []string // Variable names
	ReturnType Type
}

// ReturnInstruction represents a return instruction (e.g., `return %temp3`)
type ReturnInstruction struct {
	Value string // Variable name
}

// AllocInstruction represents an alloc instruction (e.g., `alloc Foo : ptr<Foo>`)
type AllocInstruction struct {
	VarName    string // Name of the variable to store the result
	TypeName   string
	ReturnType Type
}

// FreeInstruction represents a free instruction (e.g., `free %temp3`)
type FreeInstruction struct {
	Value string // Variable name
}

// StoreInstruction represents a store instruction (e.g., `store %temp3, %temp2`)
// or `store %temp3, %y, 1`
type StoreInstruction struct {
	Pointer   string // Variable name of the pointer
	Value     string // Variable name of the value to store
	Index     *int   // Optional index for struct field access
}

// LoadInstruction represents a load instruction (e.g., `load %ptr, %val`)
// or `load %ptr, %val, 1`
type LoadInstruction struct {
	VarName   string // Name of the variable to store the loaded value
	Pointer   string // Variable name of the pointer
	Index     *int   // Optional index for struct field access
}

// BranchInstruction represents an unconditional branch instruction (e.g., `br label0`)
type BranchInstruction struct {
	Label string
}

// ConditionalBranchInstruction represents a conditional branch instruction (e.g., `br_cond %temp1, label0, label1`)
type ConditionalBranchInstruction struct {
	Condition string // Variable name of the condition
	TrueLabel  string
	FalseLabel string
}

// Label represents a label in the IR (e.g., `label0:`)
type Label struct {
	Name string
}

// BinaryExpressionInstruction represents a binary operation that produces a value
// e.g., %temp1 = %x > %temp0 : bool
type BinaryExpressionInstruction struct {
	ResultVar string // Variable to store the result
	Operand1  string // Variable name
	Operator  string // e.g., ">", "<", "==", "!="
	Operand2  string // Variable name
	Type      Type   // Type of the result
}

// ConstantAssignmentInstruction represents assigning a constant to a variable
// e.g., %temp0 = 10 : int32
type ConstantAssignmentInstruction struct {
	VarName string
	Value   string // The string representation of the constant value
	Type    Type
}

// Ensure all AST nodes implement the Node interface
var _ Node = (*File)(nil)
var _ Node = (*TypeDefinition)(nil)
var _ Node = (*StructType)(nil)
var _ Node = (*PointerType)(nil)
var _ Node = (*BasicType)(nil)
var _ Node = (*FunctionDefinition)(nil)
var _ Node = (*VarDeclaration)(nil)
var _ Node = (Instruction)(nil)
var _ Node = (*Label)(nil)

// Ensure all instruction types implement the Instruction interface
var _ Instruction = (*CallInstruction)(nil)
var _ Instruction = (*ReturnInstruction)(nil)
var _ Instruction = (*AllocInstruction)(nil)
var _ Instruction = (*FreeInstruction)(nil)
var _ Instruction = (*StoreInstruction)(nil)
var _ Instruction = (*LoadInstruction)(nil)
var _ Instruction = (*BranchInstruction)(nil)
var _ Instruction = (*ConditionalBranchInstruction)(nil)
var _ Instruction = (*BinaryExpressionInstruction)(nil)
var _ Instruction = (*ConstantAssignmentInstruction)(nil)


// Ensure all type nodes implement the Type interface
var _ Type = (*StructType)(nil)
var _ Type = (*PointerType)(nil)
var _ Type = (*BasicType)(nil)

// Helper functions to create basic types
func NewBasicType(name string) *BasicType {
	return &BasicType{Name: name}
}

func NewPointerType(elementType Type) *PointerType {
	return &PointerType{ElementType: elementType}
}

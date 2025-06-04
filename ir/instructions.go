package ir

// Type represents a type in the IR.
type Type interface {
	isType()
	String() string
}

// SimpleType represents a basic type like int32, float64, bool.
type SimpleType struct {
	Name string
}

func (s *SimpleType) isType() {}
func (s *SimpleType) String() string {
	return s.Name
}

// PointerType represents a pointer to another type.
type PointerType struct {
	Element Type
}

func (p *PointerType) isType() {}
func (p *PointerType) String() string {
	return "ptr<" + p.Element.String() + ">"
}

// StructTypeDefinition defines a struct type.
type StructTypeDefinition struct {
	Name   string
	Fields []Type
}

func (s *StructTypeDefinition) isType() {}
func (s *StructTypeDefinition) String() string {
	// TODO: Implement a proper string representation for struct fields
	return s.Name
}

// FunctionDefinition defines a function signature.
type FunctionDefinition struct {
	Name       string
	Parameters []Variable
	ReturnType Type
	Body       []Instruction
	Parent     *Program // Reference to the parent program
}

// TopLevelDefinition is an interface for top-level definitions like structs and functions.
type TopLevelDefinition interface {
	isTopLevelDefinition()
}

func (s *StructTypeDefinition) isTopLevelDefinition() {}
func (f *FunctionDefinition) isTopLevelDefinition() {}

// Program represents the entire IR program.
type Program struct {
	Structs   map[string]*StructTypeDefinition
	Functions map[string]*FunctionDefinition
	// TODO: Add externs, globals, etc. if needed
}

// Operand represents an operand in an instruction.
type Operand interface {
	isOperand()
	String() string
}

// Variable represents a variable, like %x or %temp0.
type Variable struct {
	Name string
	Type Type
}

func (v *Variable) isOperand() {}
func (v *Variable) String() string {
	return "%" + v.Name
}

// Constant represents a constant value, like 10.
type Constant struct {
	Value string // Using string to represent any constant for now
	Type  Type
}

func (c *Constant) isOperand() {}
func (c *Constant) String() string {
	// If type is specified and not a default/generic one, print it.
	// This helps distinguish e.g. 10:int32 from just 10.
	// Default types set by parser are like "int_const_default".
	if c.Type != nil && !strings.HasSuffix(c.Type.String(), "_const_default") {
		return c.Value + " : " + c.Type.String()
	}
	return c.Value
}

// TypeName represents a type name, used in instructions.
// This might be redundant if Type interface can be used directly in instructions.
// For now, keeping it as specified.
type TypeName struct {
	Name string
}

func (t *TypeName) isOperand() {}
func (t *TypeName) String() string {
	return t.Name
}

// Instruction is the interface that all instructions implement.
type Instruction interface {
	isInstruction()
	String() string
}

// CallInstruction represents a 'call name([arg]) : type' instruction.
// Example: %result = call foobar(%x, %y) : ptr<Foo>
type CallInstruction struct {
	Target    string    // Function name
	Arguments []Operand // Arguments to the call
	// ReturnType is part of Dest.Type
	Dest Variable // Variable to store the result
}

func (i *CallInstruction) isInstruction() {}
func (i *CallInstruction) String() string {
	var args []string
	for _, arg := range i.Arguments {
		args = append(args, arg.String())
	}
	argsStr := strings.Join(args, ", ")

	// Ensure i.Dest.Type is not nil (it's the return type)
	retTypeStr := "<undefined_type>"
	if i.Dest.Type != nil {
		retTypeStr = i.Dest.Type.String()
	}
	// Format: %dest = call fn_name(arg1, arg2) : return_type
	return fmt.Sprintf("%s = call %s(%s) : %s", i.Dest.String(), i.Target, argsStr, retTypeStr)
}

// ReturnInstruction represents a 'return var' instruction.
// Example: return %temp3
type ReturnInstruction struct {
	Value Operand // Can be Variable or Constant
}

func (i *ReturnInstruction) isInstruction() {}
func (i *ReturnInstruction) String() string {
	return "return " + i.Value.String()
}

// AllocInstruction represents an 'alloc type' instruction.
// Example: alloc Foo
type AllocInstruction struct {
	Type Type     // Type to allocate
	Dest Variable // Destination variable for the pointer
}

func (i *AllocInstruction) isInstruction() {}
func (i *AllocInstruction) String() string {
	return i.Dest.String() + " = alloc " + i.Type.String()
}

// FreeInstruction represents a 'free ptr' instruction.
// Example: free %ptr
type FreeInstruction struct {
	Pointer Operand // Should be a Variable of PointerType
}

func (i *FreeInstruction) isInstruction() {}
func (i *FreeInstruction) String() string {
	return "free " + i.Pointer.String()
}

// StoreInstruction represents a 'store ptr, var, ?index' instruction.
// Example: store %temp3, %temp2
// Example: store %temp3, %y, 1
type StoreInstruction struct {
	Pointer Operand // Should be a Variable of PointerType
	Value   Operand // Can be Variable or Constant
	Index   Operand // Optional: Variable or Constant (for struct field index)
}

func (i *StoreInstruction) isInstruction() {}
func (i *StoreInstruction) String() string {
	s := "store " + i.Pointer.String() + ", " + i.Value.String()
	if i.Index != nil {
		s += ", " + i.Index.String()
	}
	return s
}

// LoadInstruction represents a 'load ptr, ?index' instruction.
// Example: %val = load %ptr
// Example: %field = load %struct_ptr, 1
type LoadInstruction struct {
	Dest    Variable // Variable to store the loaded value
	Pointer Operand  // Should be a Variable of PointerType
	Index   Operand  // Optional: Variable or Constant (for struct field index)
}

func (i *LoadInstruction) isInstruction() {}
func (i *LoadInstruction) String() string {
	s := fmt.Sprintf("%s = load %s", i.Dest.String(), i.Pointer.String())
	if i.Index != nil {
		s += ", " + i.Index.String()
	}
	return s
}

// BranchInstruction represents a 'br label' instruction.
// Example: br label0
type BranchInstruction struct {
	Label string
}

func (i *BranchInstruction) isInstruction() {}
func (i *BranchInstruction) String() string {
	return "br " + i.Label
}

// ConditionalBranchInstruction represents a 'br_cond cond, labelTrue, labelFalse' instruction.
// Example: br_cond %temp1, label0, label1
type ConditionalBranchInstruction struct {
	Condition  Operand // Should be a Variable or Constant of boolean type
	TrueLabel  string
	FalseLabel string
}

func (i *ConditionalBranchInstruction) isInstruction() {}
func (i *ConditionalBranchInstruction) String() string {
	return "br_cond " + i.Condition.String() + ", " + i.TrueLabel + ", " + i.FalseLabel
}

// Label represents a jump target like 'label0:'.
// Example: label0:
type Label struct {
	Name string
}

func (l *Label) isInstruction() {}
func (l *Label) String() string {
	return l.Name + ":"
}

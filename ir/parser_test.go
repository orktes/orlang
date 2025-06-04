package ir

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/orktes/orlang/scanner"
	// Removed: "github.com/DQNEO/babygo/internal/token"
	// ast import might be needed if Position is directly used in tests, but scanner.Token embeds it.
)

func newTestParser(input string) *Parser {
	// Assuming ir.NewScanner now correctly uses orlang/scanner.Scanner internally.
	// The orlang/scanner.NewScanner takes io.Reader. ir.NewScanner wraps this.
	// The third argument to the internal scanner.NewScanner (token.ILLEGAL) is related to its error handling,
	// which might not be directly applicable here or is handled within ir.NewScanner.
	// Let's assume ir.NewScanner is configured correctly.
	s := NewScanner(scanner.NewScanner(strings.NewReader(input))) // Simplified call if filename and error token not needed at this level by orlang/scanner
	return NewParser(s)
}

// normalizeTypeInProgram resolves SimpleType names to actual StructTypeDefinition pointers
// from the program's Structs map if they represent defined structs.
// It recursively processes PointerType elements and StructTypeDefinition fields.
func normalizeTypeInProgram(prog *Program, t Type) Type {
	if t == nil {
		return nil
	}
	switch ty := t.(type) {
	case *SimpleType:
		if s, ok := prog.Structs[ty.Name]; ok {
			return s // Replace with the program's actual struct definition
		}
		return ty // It's a primitive or unresolved external type
	case *PointerType:
		return &PointerType{Element: normalizeTypeInProgram(prog, ty.Element)}
	case *StructTypeDefinition:
		// If we are normalizing a struct definition that might be part of an expected value,
		// ensure its fields are also normalized. This is less common for expected values
		// as they usually use SimpleType for placeholders.
		normalizedFields := make([]Type, len(ty.Fields))
		for i, f := range ty.Fields {
			normalizedFields[i] = normalizeTypeInProgram(prog, f)
		}
		return &StructTypeDefinition{Name: ty.Name, Fields: normalizedFields}
	default:
		return t // Should not happen for valid types
	}
}

// normalizeOperand updates the types within an operand (Variable or Constant)
// to use resolved struct pointers from the program.
func normalizeOperand(prog *Program, op Operand) Operand {
	if op == nil {
		return nil
	}
	switch o := op.(type) {
	case *Variable:
		vCopy := *o
		vCopy.Type = normalizeTypeInProgram(prog, vCopy.Type)
		return &vCopy
	case *Constant:
		cCopy := *o
		cCopy.Type = normalizeTypeInProgram(prog, cCopy.Type)
		return &cCopy
	default:
		return o // Other operand types if any
	}
}

// normalizeInstruction updates types within an instruction (in Dest, Operands, etc.)
// to use resolved struct pointers from the program.
// It returns a copy of the instruction with normalized types.
func normalizeInstruction(prog *Program, instr Instruction) Instruction {
	if instr == nil {
		return nil
	}
	// Create a clone to modify, assumes Clone() method exists and makes shallow copy of types/operands
	clonedInstr := instr.(Cloner).Clone() // Add Cloner interface to all instructions

	switch i := clonedInstr.(type) {
	// VarInstruction case removed
	case *AllocInstruction:
		i.Type = normalizeTypeInProgram(prog, i.Type)
		i.Dest.Type = normalizeTypeInProgram(prog, i.Dest.Type)
	case *LoadInstruction:
		i.Pointer = normalizeOperand(prog, i.Pointer)
		if i.Index != nil {
			i.Index = normalizeOperand(prog, i.Index)
		}
		i.Dest.Type = normalizeTypeInProgram(prog, i.Dest.Type)
	case *CallInstruction:
		for j, arg := range i.Arguments {
			i.Arguments[j] = normalizeOperand(prog, arg)
		}
		i.Dest.Type = normalizeTypeInProgram(prog, i.Dest.Type)
	case *StoreInstruction:
		i.Pointer = normalizeOperand(prog, i.Pointer)
		i.Value = normalizeOperand(prog, i.Value)
		if i.Index != nil {
			i.Index = normalizeOperand(prog, i.Index)
		}
	case *ReturnInstruction:
		i.Value = normalizeOperand(prog, i.Value)
	case *FreeInstruction:
		i.Pointer = normalizeOperand(prog, i.Pointer)
	case *ConditionalBranchInstruction:
		i.Condition = normalizeOperand(prog, i.Condition)
	case *BranchInstruction, *Label:
		// No types to normalize
	default:
		panic(fmt.Sprintf("unhandled instruction type for normalization: %T", i))
	}
	return clonedInstr
}

func TestParseStructTypeDefinition(t *testing.T) {
	input := "type Foo {int32, ptr<Bar>}\ntype Bar {float64}"
	parser := newTestParser(input)
	program := parser.Parse()

	if len(program.Structs) != 2 {
		t.Fatalf("expected 2 struct definitions, got %d", len(program.Structs))
	}
	foo, okFoo := program.Structs["Foo"]
	bar, okBar := program.Structs["Bar"]
	if !okFoo || !okBar {
		t.Fatalf("expected structs 'Foo' and 'Bar' not found")
	}

	expectedFooFields := []Type{
		&SimpleType{Name: "int32"},
		&PointerType{Element: bar}, // Bar is resolved to the actual *StructTypeDefinition
	}
	if !reflect.DeepEqual(foo.Fields, expectedFooFields) {
		t.Errorf("Foo: expected fields %#v, got %#v", expectedFooFields, foo.Fields)
	}

	inputEmpty := "type Empty {}"
	parserEmpty := newTestParser(inputEmpty)
	programEmpty := parserEmpty.Parse()
	emptyStruct, _ := programEmpty.Structs["Empty"]
	if len(emptyStruct.Fields) != 0 {
		t.Errorf("expected 0 fields for Empty struct, got %d", len(emptyStruct.Fields))
	}
}

func TestParseFunctionDefinition(t *testing.T) {
	input := `
type Foo {}
fn my_func(%a : int32, %b : ptr<Foo>) : bool {
  label_entry:
  // var %x : int32 // This line is removed
  %y = alloc Foo
  return %a
}
`
	parser := newTestParser(input)
	program := parser.Parse()

	if len(program.Functions) != 1 {
		t.Fatalf("expected 1 function definition, got %d", len(program.Functions))
	}
	f, ok := program.Functions["my_func"]
	if !ok {
		t.Fatalf("expected function 'my_func' not found")
	}

	fooStruct := program.Structs["Foo"] // Will be resolved to the actual *StructTypeDefinition
	expectedParams := []Variable{
		{Name: "a", Type: &SimpleType{Name: "int32"}},
		{Name: "b", Type: &PointerType{Element: fooStruct}},
	}

	if len(f.Parameters) != len(expectedParams) {
		t.Fatalf("param length mismatch: expected %d, got %d", len(expectedParams), len(f.Parameters))
	}
	for i := range expectedParams {
		normalizedExpectedParamType := normalizeTypeInProgram(program, expectedParams[i].Type)
		if f.Parameters[i].Name != expectedParams[i].Name {
			t.Errorf("Param %d name: expected %s, got %s", i, expectedParams[i].Name, f.Parameters[i].Name)
		}
		if !reflect.DeepEqual(f.Parameters[i].Type, normalizedExpectedParamType) {
			t.Errorf("Param %d type: expected %#v, got %#v", i, normalizedExpectedParamType, f.Parameters[i].Type)
		}
	}

	if rt, ok := f.ReturnType.(*SimpleType); !ok || rt.Name != "bool" {
		t.Errorf("expected return type 'bool', got %s", f.ReturnType.String())
	}

	// Expected instructions: label_entry, %y = alloc Foo, return %a
	if len(f.Body) != 3 {
		t.Fatalf("expected 3 instructions in body, got %d. Body:\n%s", len(f.Body), f.StringAllInstructions())
	}

	// Check instruction 0: label_entry
	if label, ok := f.Body[0].(*Label); !ok || label.Name != "label_entry" {
		t.Errorf("Expected instruction 0 to be Label 'label_entry', got %T with text '%s'", f.Body[0], f.Body[0].String())
	}

	// Check instruction 1: %y = alloc Foo
	if allocInstr, ok := f.Body[1].(*AllocInstruction); !ok {
		t.Errorf("Expected instruction 1 to be AllocInstruction, got %T", f.Body[1])
	} else {
		if allocInstr.Dest.Name != "y" {
			t.Errorf("Expected AllocInstruction dest to be 'y', got '%s'", allocInstr.Dest.Name)
		}
		// Ensure the type allocated is the resolved Foo struct
		if !reflect.DeepEqual(allocInstr.Type, fooStruct) {
			t.Errorf("Expected AllocInstruction type to be '%s', got '%s'", fooStruct.Name, allocInstr.Type.String())
		}
		// Check if %y was added to localVars
		if v, exists := f.localVars["y"]; !exists {
			t.Errorf("Expected %%y to be declared in localVars after alloc instruction")
		} else {
			expectedVarType := &PointerType{Element: fooStruct}
			if !reflect.DeepEqual(v.Type, expectedVarType) {
				t.Errorf("Expected %%y in localVars to have type %s, got %s", expectedVarType.String(), v.Type.String())
			}
		}
	}

	// Check instruction 2: return %a
	if returnInstr, ok := f.Body[2].(*ReturnInstruction); !ok {
		t.Errorf("Expected instruction 2 to be ReturnInstruction, got %T", f.Body[2])
	} else {
		if retVar, ok := returnInstr.Value.(*Variable); !ok || retVar.Name != "a" {
			t.Errorf("Expected ReturnInstruction value to be '%%a', got '%s'", returnInstr.Value.String())
		}
	}
}

func TestParseInstructions(t *testing.T) {
	typeSetup := "type MyStruct {int32, bool}\n"
	makeInput := func(instrStr string) string {
		return typeSetup + `fn test_instr_func(%param1 : int32, %ptr_param : ptr<MyStruct>, %bool_param : bool) : void {
` + instrStr + `
}`
	}

	// MyStruct as it would be after parsing and available in Program.Structs
	myStructDef := &StructTypeDefinition{
		Name:   "MyStruct",
		Fields: []Type{&SimpleType{Name: "int32"}, &SimpleType{Name: "bool"}},
	}

	tests := []struct {
		name          string
		instrStr      string
		expectedInstr Instruction // Expected instruction structure
	}{
		// VarInstruction test case removed:
		// {"var", "var %my_var : ptr<float64>", &VarInstruction{Dest: Variable{Name: "my_var", Type: &PointerType{Element: &SimpleType{Name: "float64"}}}}},
		{"alloc", "%r = alloc MyStruct", &AllocInstruction{Dest: Variable{Name: "r", Type: &PointerType{Element: myStructDef}}, Type: myStructDef}},
		{"load direct", "%r = load %ptr_param", &LoadInstruction{Dest: Variable{Name: "r", Type: myStructDef}, Pointer: &Variable{Name: "ptr_param", Type: &PointerType{Element: myStructDef}}}},
		{"load indexed", "%r = load %ptr_param, 0 : int32", &LoadInstruction{Dest: Variable{Name: "r", Type: &SimpleType{Name: "int32"}}, Pointer: &Variable{Name: "ptr_param", Type: &PointerType{Element: myStructDef}}, Index: &Constant{Value: "0", Type: &SimpleType{Name: "int32"}}}},
		{"call no args", "%r = call f1() : MyStruct", &CallInstruction{Target: "f1", Arguments: []Operand{}, Dest: Variable{Name: "r", Type: myStructDef}}},
		{"call w args", "%r = call f2(%param1, 10 :custom, %bool_param) : int32", &CallInstruction{Target: "f2", Arguments: []Operand{&Variable{Name: "param1", Type: &SimpleType{"int32"}}, &Constant{Value: "10", Type: &SimpleType{"custom"}}, &Variable{Name: "bool_param", Type: &SimpleType{"bool"}}}, Dest: Variable{Name: "r", Type: &SimpleType{"int32"}}}},
		{"store direct", "store %ptr_param, %param1", &StoreInstruction{Pointer: &Variable{Name: "ptr_param", Type: &PointerType{Element: myStructDef}}, Value: &Variable{Name: "param1", Type: &SimpleType{"int32"}}}}, // This should be field 0
		{"store indexed", "store %ptr_param, %bool_param, 1:int32", &StoreInstruction{Pointer: &Variable{Name: "ptr_param", Type: &PointerType{Element: myStructDef}}, Value: &Variable{Name: "bool_param", Type: &SimpleType{"bool"}}, Index: &Constant{Value: "1", Type: &SimpleType{"int32"}}}},
		{"return", "return %param1", &ReturnInstruction{Value: &Variable{Name: "param1", Type: &SimpleType{"int32"}}}},
		{"free", "free %ptr_param", &FreeInstruction{Pointer: &Variable{Name: "ptr_param", Type: &PointerType{Element: myStructDef}}}},
		{"br", "br L1", &BranchInstruction{Label: "L1"}},
		{"br_cond", "br_cond %bool_param, L1, L2", &ConditionalBranchInstruction{Condition: &Variable{Name: "bool_param", Type: &SimpleType{"bool"}}, TrueLabel: "L1", FalseLabel: "L2"}},
		{"label", "L1:", &Label{Name: "L1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fullInput := makeInput(tt.instrStr)
			parser := newTestParser(fullInput)
			program := parser.Parse() // Parses typeSetup and the function

			fn := program.Functions["test_instr_func"]
			if fn == nil || len(fn.Body) == 0 {
				t.Fatalf("Function 'test_instr_func' or its body not parsed correctly. Input:\n%s", fullInput)
			}
			parsedInstr := fn.Body[0]

			// Normalize the expected instruction using the program context
			normalizedExpectedInstr := normalizeInstruction(program, tt.expectedInstr)

			if !reflect.DeepEqual(parsedInstr, normalizedExpectedInstr) {
				t.Errorf("Instruction mismatch for '%s':\nExpected: %#v\nGot:      %#v", tt.instrStr, normalizedExpectedInstr, parsedInstr)
				// Log sub-field differences if useful
				if allocInstr, ok := parsedInstr.(*AllocInstruction); ok {
					expectedAllocInstr, okExpected := normalizedExpectedInstr.(*AllocInstruction)
					if okExpected { // Ensure the expected instruction is also an AllocInstruction
						if !reflect.DeepEqual(allocInstr.Dest.Type, expectedAllocInstr.Dest.Type) {
							t.Logf("Alloc Dest Type Diff:\nExp: %#v\nGot: %#v", expectedAllocInstr.Dest.Type, allocInstr.Dest.Type)
						}
					}
				}
			}
		})
	}
}

func TestParseFullProgramSpecExample(t *testing.T) {
	input := `
type Foo {int32, int32}

// Dummy function for the call in the test, so parser knows about it if it checks.
// However, our parser does not pre-validate function calls against existing FunctionDefinitions.
fn check_condition(%val : int32) : bool {
	return %val // Body not semantically checked by this IR parser phase
}

fn foobar(%x : int32, %y : int32) : ptr<Foo> {
  %cond_val = call check_condition(%x) : bool // Implicit declaration of %cond_val
  br_cond %cond_val, label0, label1

label0:
  %temp3 = alloc Foo                     // Implicit declaration of %temp3
  store %temp3, 10 : int32, 0 : int32   // Use constant 10 directly
  store %temp3, %y, 1 : int32
  return %temp3

label1:
  %temp4 = alloc Foo                     // Implicit declaration of %temp4
  store %temp4, %x, 0 : int32
  store %temp4, %y, 1 : int32
  return %temp4
}
`
	parser := newTestParser(input)
	program := parser.Parse()

	if _, ok := program.Structs["Foo"]; !ok {
		t.Error("Expected struct 'Foo' not found")
	}
	fnFoobar, okFoobar := program.Functions["foobar"]
	if !okFoobar {
		t.Fatal("Expected function 'foobar' not found")
	}
	fnCheckCondition, okCheckCondition := program.Functions["check_condition"]
	if !okCheckCondition {
		t.Fatal("Expected function 'check_condition' not found")
	}


	fooStruct := program.Structs["Foo"]
	expectedFoobarParams := []Variable{
		{Name: "x", Type: &SimpleType{Name: "int32"}},
		{Name: "y", Type: &SimpleType{Name: "int32"}},
	}
	if !reflect.DeepEqual(fnFoobar.Parameters, expectedFoobarParams) {
		t.Errorf("Parameters mismatch for foobar:\nExpected: %#v\nGot:      %#v", expectedFoobarParams, fnFoobar.Parameters)
	}
	expectedFoobarRetType := &PointerType{Element: fooStruct}
	if !reflect.DeepEqual(fnFoobar.ReturnType, normalizeTypeInProgram(program, expectedFoobarRetType)) {
		t.Errorf("Return type mismatch for foobar: expected %s, got %s",
			PrintType(normalizeTypeInProgram(program, expectedFoobarRetType)),
			PrintType(fnFoobar.ReturnType))
	}

	// Expected instruction count for foobar:
	// call, br_cond, label0, alloc, store, store, return, label1, alloc, store, store, return = 12
	if len(fnFoobar.Body) != 12 {
		t.Fatalf("Expected 12 instructions in foobar, got %d. Body:\n%s", len(fnFoobar.Body), fnFoobar.StringAllInstructions())
	}

	// Spot check a few instructions and verify variable declarations
	// 1. %cond_val = call check_condition(%x) : bool
	expectedCall := &CallInstruction{
		Target:    "check_condition",
		Arguments: []Operand{&Variable{Name: "x", Type: &SimpleType{Name: "int32"}}}, // %x is a param
		Dest:      Variable{Name: "cond_val", Type: &SimpleType{Name: "bool"}},
	}
	normalizedExpectedCall := normalizeInstruction(program, expectedCall)
	if !reflect.DeepEqual(fnFoobar.Body[0], normalizedExpectedCall) {
		t.Errorf("Call instruction mismatch:\nExpected: %#v\nGot:      %#v",
			normalizedExpectedCall, fnFoobar.Body[0])
	}
	if v, exists := fnFoobar.localVars["cond_val"]; !exists {
		t.Errorf("Expected %%cond_val to be declared in localVars after call instruction")
	} else {
		if !reflect.DeepEqual(v.Type, &SimpleType{Name: "bool"}) {
			t.Errorf("Expected %%cond_val to have type bool, got %s", v.Type.String())
		}
	}


	// 2. br_cond %cond_val, label0, label1
	expectedBrCond := &ConditionalBranchInstruction{
		Condition:  &Variable{Name: "cond_val", Type: &SimpleType{Name: "bool"}}, // Type from the call result
		TrueLabel:  "label0",
		FalseLabel: "label1",
	}
	if !reflect.DeepEqual(fnFoobar.Body[1], normalizeInstruction(program, expectedBrCond)) {
		t.Errorf("br_cond instruction mismatch:\nExpected: %#v\nGot:      %#v",
			normalizeInstruction(program, expectedBrCond), fnFoobar.Body[1])
	}

	// 3. %temp3 = alloc Foo (in label0 block, index 3: label0 is at 2)
	expectedAlloc := &AllocInstruction{
		Dest: Variable{Name: "temp3", Type: &PointerType{Element: fooStruct}}, // fooStruct is already resolved
		Type: fooStruct,
	}
	normalizedExpectedAlloc := normalizeInstruction(program, expectedAlloc) // Normalization might adjust internal pointers if fooStruct was simple
	if !reflect.DeepEqual(fnFoobar.Body[3], normalizedExpectedAlloc) {
		t.Errorf("Alloc instruction (temp3) mismatch:\nExpected: %#v\nGot:      %#v",
			normalizedExpectedAlloc, fnFoobar.Body[3])
	}
    if v, exists := fnFoobar.localVars["temp3"]; !exists {
        t.Errorf("Expected %%temp3 to be declared in localVars after alloc instruction")
    } else {
        if !reflect.DeepEqual(v.Type, &PointerType{Element: fooStruct}) {
             t.Errorf("Expected %%temp3 to have type ptr<Foo>, got %s", v.Type.String())
        }
    }
}


// --- Cloner Interface and Implementations (needed for normalizeInstruction) ---
type Cloner interface {
	Clone() Instruction
}

// func (i *VarInstruction) Clone() Instruction { c := *i; c.Dest = *(i.Dest.cloneVariable()); return &c } // VarInstruction removed
func (i *CallInstruction) Clone() Instruction {
	c := *i; c.Dest = *(i.Dest.cloneVariable())
	c.Arguments = make([]Operand, len(i.Arguments));
	for j, arg := range i.Arguments { c.Arguments[j] = arg.(operandCloner).cloneOperand() }
	return &c
}
func (i *ReturnInstruction) Clone() Instruction { c := *i; c.Value = i.Value.(operandCloner).cloneOperand(); return &c }
func (i *AllocInstruction) Clone() Instruction { c := *i; c.Dest = *(i.Dest.cloneVariable()); return &c }
func (i *FreeInstruction) Clone() Instruction { c := *i; c.Pointer = i.Pointer.(operandCloner).cloneOperand(); return &c }
func (i *StoreInstruction) Clone() Instruction {
	c := *i; c.Pointer = i.Pointer.(operandCloner).cloneOperand(); c.Value = i.Value.(operandCloner).cloneOperand()
	if i.Index != nil { c.Index = i.Index.(operandCloner).cloneOperand() }
	return &c
}
func (i *LoadInstruction) Clone() Instruction {
	c := *i; c.Dest = *(i.Dest.cloneVariable()); c.Pointer = i.Pointer.(operandCloner).cloneOperand()
	if i.Index != nil { c.Index = i.Index.(operandCloner).cloneOperand() }
	return &c
}
func (i *BranchInstruction) Clone() Instruction          { c := *i; return &c }
func (i *ConditionalBranchInstruction) Clone() Instruction { c := *i; c.Condition = i.Condition.(operandCloner).cloneOperand(); return &c }
func (i *Label) Clone() Instruction                      { c := *i; return &c }

type operandCloner interface { cloneOperand() Operand }
func (v *Variable) cloneOperand() Operand { vv := *v; return &vv } // Type is shallow copied
func (c *Constant) cloneOperand() Operand { cc := *c; return &cc } // Type is shallow copied
func (t *TypeName) cloneOperand() Operand { return &TypeName{Name: t.Name} } // Should not be an operand in this way

func (v *Variable) cloneVariable() *Variable { vv := *v; return &vv} // For Dest fields

// Helper for FunctionDefinition to print all its instructions (for debugging tests)
func (f *FunctionDefinition) StringAllInstructions() string {
	var sb strings.Builder
	for _, instr := range f.Body {
		sb.WriteString(instr.String())
		sb.WriteString("\n")
	}
	return sb.String()
}
// strconv import for Atoi (used in parser, but good to have for tests if needed)
var _ = strconv.Atoi
```

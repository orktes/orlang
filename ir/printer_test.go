package ir

import (
	"strings"
	"testing"
)

func TestPrintStructTypeDefinition(t *testing.T) {
	s := &StructTypeDefinition{
		Name: "MyStruct",
		Fields: []Type{
			&SimpleType{Name: "int32"},
			&PointerType{Element: &SimpleType{Name: "Other"}},
			&SimpleType{Name: "bool"},
		},
	}
	expected := "type MyStruct {int32, ptr<Other>, bool}"
	if got := PrintStructTypeDefinition(s); got != expected {
		t.Errorf("PrintStructTypeDefinition:\nExpected: %s\nGot:      %s", expected, got)
	}

	sEmpty := &StructTypeDefinition{Name: "EmptyStruct", Fields: []Type{}}
	expectedEmpty := "type EmptyStruct {}"
	if got := PrintStructTypeDefinition(sEmpty); got != expectedEmpty {
		t.Errorf("PrintStructTypeDefinition (empty):\nExpected: %s\nGot:      %s", expectedEmpty, got)
	}
}

func TestPrintFunctionDefinition(t *testing.T) {
	fooStruct := &StructTypeDefinition{Name: "Foo", Fields: []Type{}}
	f := &FunctionDefinition{
		Name: "my_func",
		Parameters: []Variable{
			{Name: "count", Type: &SimpleType{Name: "int32"}},
			{Name: "data", Type: &PointerType{Element: fooStruct}},
		},
		ReturnType: &SimpleType{Name: "void"},
		Body: []Instruction{
			&Label{Name: "entry"},
			// VarInstruction removed, replaced with an alloc for testing purposes to have another instruction.
			&AllocInstruction{
				Dest: Variable{Name: "temp_var_alloc", Type: &PointerType{Element: &SimpleType{Name: "float64"}}},
				Type: &SimpleType{Name: "float64"},
			},
			&AllocInstruction{
				Dest: Variable{Name: "alloc_ptr", Type: &PointerType{Element: fooStruct}},
				Type: fooStruct,
			},
			&ReturnInstruction{Value: &Constant{Value: "0", Type: &SimpleType{Name: "int32"}}},
		},
	}

	expected := `fn my_func(%count : int32, %data : ptr<Foo>) : void {
entry:
  %temp_var_alloc = alloc float64
  %alloc_ptr = alloc Foo
  return 0 : int32
}`
	// Normalize newlines for comparison, as expected string uses \n from literal
	expected = strings.ReplaceAll(expected, "\r\n", "\n")
	got := PrintFunctionDefinition(f)
	got = strings.ReplaceAll(got, "\r\n", "\n")


	if got != expected {
		t.Errorf("PrintFunctionDefinition:\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}

func TestPrintInstructions(t *testing.T) {
	myStruct := &StructTypeDefinition{Name: "MyS", Fields: []Type{&SimpleType{"f1"}, &SimpleType{"f2"}}}
	ptrToMyStruct := &PointerType{Element: myStruct}

	tests := []struct {
		name     string
		instr    Instruction
		expected string
	}{
		// {"var", &VarInstruction{Dest: Variable{Name: "x", Type: &SimpleType{"int64"}}}, "var %x : int64"}, // VarInstruction removed
		{"alloc", &AllocInstruction{Dest: Variable{Name: "p1", Type: ptrToMyStruct}, Type: myStruct}, "%p1 = alloc MyS"},
		{"load direct", &LoadInstruction{Dest: Variable{Name: "ldval", Type: myStruct}, Pointer: &Variable{Name: "p1", Type: ptrToMyStruct}}, "%ldval = load %p1"},
		{"load indexed", &LoadInstruction{Dest: Variable{Name: "fld", Type: &SimpleType{"f1"}}, Pointer: &Variable{Name: "p1", Type: ptrToMyStruct}, Index: &Constant{Value: "0", Type: &SimpleType{"int32"}}}, "%fld = load %p1, 0 : int32"},
		{"store direct", &StoreInstruction{Pointer: &Variable{Name: "p1", Type: ptrToMyStruct}, Value: &Variable{Name: "ldval", Type: myStruct}}, "store %p1, %ldval"},
		{"store indexed", &StoreInstruction{Pointer: &Variable{Name: "p1", Type: ptrToMyStruct}, Value: &Variable{Name: "sval", Type: &SimpleType{"f1"}}, Index: &Constant{Value: "0"}}, "store %p1, %sval, 0"}, // Constant type not printed if default
		{"call no args", &CallInstruction{Target: "do_work", Dest: Variable{Name: "res", Type: &SimpleType{"bool"}}}, "%res = call do_work() : bool"},
		{"call w args", &CallInstruction{Target: "add", Arguments: []Operand{&Variable{Name: "a", Type: &SimpleType{"int32"}}, &Constant{Value: "55", Type: &SimpleType{"int32"}}}, Dest: Variable{Name: "sum", Type: &SimpleType{"int32"}}}, "%sum = call add(%a, 55 : int32) : int32"},
		{"return", &ReturnInstruction{Value: &Variable{Name: "res"}}, "return %res"}, // Variable type not printed here
		{"free", &FreeInstruction{Pointer: &Variable{Name: "p1"}}, "free %p1"},
		{"br", &BranchInstruction{Label: "LOOP_HEAD"}, "br LOOP_HEAD"},
		{"br_cond", &ConditionalBranchInstruction{Condition: &Variable{Name: "cond", Type: &SimpleType{"bool"}}, TrueLabel: "IF_TRUE", FalseLabel: "IF_FALSE"}, "br_cond %cond, IF_TRUE, IF_FALSE"},
		{"label", &Label{Name: "END_BLOCK"}, "END_BLOCK:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// PrintInstruction just calls instr.String(), so this tests the String() methods
			if got := PrintInstruction(tt.instr); got != tt.expected {
				t.Errorf("PrintInstruction (%s):\nExpected: %s\nGot:      %s", tt.name, tt.expected, got)
			}
		})
	}
}

func TestPrintProgram(t *testing.T) {
	fooStruct := &StructTypeDefinition{Name: "Foo", Fields: []Type{&SimpleType{"int32"}}}
	barStruct := &StructTypeDefinition{Name: "Bar", Fields: []Type{&SimpleType{"float64"}, &PointerType{Element: fooStruct}}}

	prog := &Program{
		Structs: map[string]*StructTypeDefinition{
			"Foo": fooStruct,
			"Bar": barStruct,
		},
		Functions: map[string]*FunctionDefinition{
			"my_func": {
				Name:       "my_func",
				Parameters: []Variable{{Name: "b", Type: barStruct}},
				ReturnType: &PointerType{Element: fooStruct},
				Body: []Instruction{
					&AllocInstruction{Dest: Variable{Name: "foo_ptr", Type: &PointerType{Element: fooStruct}}, Type: fooStruct},
					&ReturnInstruction{Value: &Variable{Name: "foo_ptr"}},
				},
			},
			"another_func": {
				Name:       "another_func",
				Parameters: []Variable{},
				ReturnType: &SimpleType{"void"},
				Body:       []Instruction{&Label{Name: "start"}, &ReturnInstruction{}},
			},
		},
	}

	// Expected output - functions and structs are sorted by name by PrintProgram
	expected := `type Bar {float64, ptr<Foo>}

type Foo {int32}

fn another_func() : void {
start:
  return
}

fn my_func(%b : Bar) : ptr<Foo> {
  %foo_ptr = alloc Foo
  return %foo_ptr
}`
	// Normalize newlines for comparison
	expected = strings.ReplaceAll(expected, "\r\n", "\n")
	got := PrintProgram(prog)
	got = strings.ReplaceAll(got, "\r\n", "\n")


	if got != expected {
		t.Errorf("PrintProgram:\nExpected:\n%s\nGot:\n%s", expected, got)
	}
}

// Test case for Constant.String() specifically
func TestPrintConstantWithType(t *testing.T) {
	c1 := &Constant{Value: "123", Type: &SimpleType{Name: "int32"}}
	expected1 := "123 : int32"
	if c1.String() != expected1 {
		t.Errorf("Constant with type: expected '%s', got '%s'", expected1, c1.String())
	}

	c2 := &Constant{Value: "true", Type: &SimpleType{Name: "bool"}}
	expected2 := "true : bool" // Assuming "bool" is not a "_const_default"
	if c2.String() != expected2 {
		t.Errorf("Constant with type (bool): expected '%s', got '%s'", expected2, c2.String())
	}

	// Test for default/inferred types that should not print the type
	c3 := &Constant{Value: "456", Type: &SimpleType{Name: "int_const_default"}}
	expected3 := "456"
	if c3.String() != expected3 {
		t.Errorf("Constant with default type: expected '%s', got '%s'", expected3, c3.String())
	}

    c4 := &Constant{Value: "78.9", Type: &SimpleType{Name: "float_const_default"}}
	expected4 := "78.9"
	if c4.String() != expected4 {
		t.Errorf("Constant with default float type: expected '%s', got '%s'", expected4, c4.String())
	}

	c5 := &Constant{Value: "nullptr", Type: &PointerType{Element: &SimpleType{"void"}}} // Example of a typed null
	expected5 := "nullptr : ptr<void>"
	if c5.String() != expected5 {
		t.Errorf("Constant with pointer type: expected '%s', got '%s'", expected5, c5.String())
	}
}

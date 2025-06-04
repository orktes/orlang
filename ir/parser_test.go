package ir

import (
	"strings"
	"testing"

	"github.com/orktes/orlang/scanner"
)

func TestParser_SpecExample(t *testing.T) {
	input := `
 // Type definition from spec
 type Foo {int32, int32}

 // Function definition from spec
 fn foobar(%x : int32, %y : int32) : ptr<Foo> {
 label_pre_cond:
  %temp0 = 10 : int32
  %temp1 = %x > %temp0 : bool

  br_cond %temp1, label0, label1

 label0: // comment for label0
  %temp2 = 10 : int32
  %temp3 = alloc Foo : ptr<Foo>

  store %temp3, %temp2 // Store x (or 10) into first field
  store %temp3, %y, 1 // Store y into second field

  return %temp3 : ptr<Foo>

 label1:
  %temp4 = alloc Foo : ptr<Foo>

  store %temp4, %x // Store x into first field (original x)
  store %temp4, %y, 1 // Store y into second field

  return %temp4 : ptr<Foo>
 }
 `

	// Create a new IR scanner (which wraps the main Orlang scanner)
	irScanner := NewScanner(scanner.NewScanner(strings.NewReader(input)))

	file, err := Parse(irScanner)

	if err != nil {
		t.Fatalf("Parse() error = %v, want nil", err)
	}

	if file == nil {
		t.Fatalf("Parse() returned nil file, want non-nil")
	}

	// Basic checks on the parsed structure
	if len(file.Nodes) != 2 {
		t.Errorf("len(file.Nodes) = %d, want 2 (1 type def, 1 func def)", len(file.Nodes))
	}

	// Check TypeDefinition
	typeDef, ok := file.Nodes[0].(*TypeDefinition)
	if !ok {
		t.Fatalf("file.Nodes[0] is not *TypeDefinition, got %T", file.Nodes[0])
	}
	if typeDef.Name != "Foo" {
		t.Errorf("typeDef.Name = %s, want Foo", typeDef.Name)
	}
	structType, ok := typeDef.Type.(*StructType)
	if !ok {
		t.Fatalf("typeDef.Type is not *StructType, got %T", typeDef.Type)
	}
	if len(structType.Fields) != 2 {
		t.Errorf("len(structType.Fields) = %d, want 2", len(structType.Fields))
	}
	if базовыйТип, ok := structType.Fields[0].(*BasicType); !ok || базовыйТип.Name != "int32" {
		t.Errorf("Field 0 type = %T name %s, want *BasicType with name int32", structType.Fields[0], базовыйТип.Name)
	}
	if базовыйТип, ok := structType.Fields[1].(*BasicType); !ok || базовыйТип.Name != "int32" {
		t.Errorf("Field 1 type = %T name %s, want *BasicType with name int32", structType.Fields[1], базовыйТип.Name)
	}

	// Check FunctionDefinition
	fnDef, ok := file.Nodes[1].(*FunctionDefinition)
	if !ok {
		t.Fatalf("file.Nodes[1] is not *FunctionDefinition, got %T", file.Nodes[1])
	}
	if fnDef.Name != "foobar" {
		t.Errorf("fnDef.Name = %s, want foobar", fnDef.Name)
	}
	if len(fnDef.Parameters) != 2 {
		t.Errorf("len(fnDef.Parameters) = %d, want 2", len(fnDef.Parameters))
	}
	// TODO: Add more detailed checks for parameters, return type, labels, and instructions

	// Test printer output (simple round-trip check, may not be identical due to formatting differences)
	// but it's a good way to see what the parser produced via the printer.
	printer := NewPrinter()
	printedIR := printer.PrintFile(file)
	t.Logf("Original IR:\n%s", input)
	t.Logf("Printed IR:\n%s", printedIR)

	// A more robust test would compare specific parts of the printed output
	// or compare the AST structure more deeply.
	// For now, we just ensure it parses and prints without crashing and perform basic structural checks.

    // Example check for a label
    if _, exists := fnDef.Labels["label0"]; !exists {
        t.Errorf("Function 'foobar' should have 'label0', but it's missing")
    }
    if _, exists := fnDef.Labels["label1"]; !exists {
        t.Errorf("Function 'foobar' should have 'label1', but it's missing")
    }
    if _, exists := fnDef.Labels["label_pre_cond"]; !exists {
        t.Errorf("Function 'foobar' should have 'label_pre_cond', but it's missing")
    }

    // Example check for instruction count (approximate)
    // The spec example has 4 instructions before label0, 5 in label0 block, 4 in label1 block.
    // This count can be tricky due to how labels are handled (in map vs. in body list).
    // Current parser stores labels in map and does not add them to instruction body.
    // Original example: 2 assignments, 1 br_cond (before label0) = 3
    // label0: 2 assignments, 1 alloc, 2 stores, 1 return = 6
    // label1: 1 alloc, 2 stores, 1 return = 4
    // Total = 3 + 6 + 4 = 13 instructions
    expectedInstructionCount := 13
    if len(fnDef.Body) != expectedInstructionCount {
        t.Errorf("Function 'foobar' has %d instructions, want %d", len(fnDef.Body), expectedInstructionCount)
        // For debugging, print the instructions if count mismatches:
        for idx, instr := range fnDef.Body {
            p := NewPrinter() // Use a new printer for just the instruction
            p.printInstruction(instr)
            t.Logf("Instruction %d: %s", idx, p.buf.String())
        }
    }

}

// TODO: Add more test cases:
// - Simpler function, type definitions.
// - All instruction types.
// - Edge cases (empty files, empty structs, empty function bodies).
// - Files with only type definitions or only function definitions.
// - Error cases (syntax errors in IR).

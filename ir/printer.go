package ir

import (
	"fmt"
	"sort"
	"strings"
)

const indent = "  " // Indentation for instructions within functions

// PrintProgram generates a string representation of the entire IR program.
func PrintProgram(prog *Program) string {
	var sb strings.Builder

	// Sort struct keys for deterministic output
	structKeys := make([]string, 0, len(prog.Structs))
	for k := range prog.Structs {
		structKeys = append(structKeys, k)
	}
	sort.Strings(structKeys)

	for i, key := range structKeys {
		s := prog.Structs[key]
		sb.WriteString(PrintStructTypeDefinition(s))
		if i < len(structKeys)-1 || len(prog.Functions) > 0 {
			sb.WriteString("\n\n")
		}
	}

	// Sort function keys for deterministic output
	funcKeys := make([]string, 0, len(prog.Functions))
	for k := range prog.Functions {
		funcKeys = append(funcKeys, k)
	}
	sort.Strings(funcKeys)

	for i, key := range funcKeys {
		f := prog.Functions[key]
		sb.WriteString(PrintFunctionDefinition(f))
		if i < len(funcKeys)-1 {
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

// PrintStructTypeDefinition generates a string representation of a struct type definition.
// Example: type Foo {int32, int32}
func PrintStructTypeDefinition(s *StructTypeDefinition) string {
	var fieldStrings []string
	for _, fieldType := range s.Fields {
		fieldStrings = append(fieldStrings, fieldType.String())
	}
	return fmt.Sprintf("type %s {%s}", s.Name, strings.Join(fieldStrings, ", "))
}

// PrintFunctionDefinition generates a string representation of a function definition.
// Example: fn foobar(%x : int32, %y : int32) : ptr<Foo> {
//   instr1
//   instr2
// }
func PrintFunctionDefinition(f *FunctionDefinition) string {
	var sb strings.Builder

	var paramStrings []string
	for _, param := range f.Parameters {
		paramStrings = append(paramStrings, fmt.Sprintf("%s : %s", param.String(), param.Type.String()))
	}

	sb.WriteString(fmt.Sprintf("fn %s(%s) : %s {\n", f.Name, strings.Join(paramStrings, ", "), f.ReturnType.String()))

	for _, instr := range f.Body {
		if _, isLabel := instr.(*Label); isLabel {
			// Labels are not indented
			sb.WriteString(instr.String())
		} else {
			sb.WriteString(indent)
			sb.WriteString(instr.String())
		}
		sb.WriteString("\n")
	}

	sb.WriteString("}")
	return sb.String()
}

// PrintInstruction generates a string representation of a single instruction.
// This mostly relies on the String() method of the instruction itself.
func PrintInstruction(instr Instruction) string {
	return instr.String()
}

// PrintType generates a string representation of a type.
// This relies on the String() method of the type itself.
func PrintType(t Type) string {
	return t.String()
}

// PrintOperand generates a string representation of an operand.
// This relies on the String() method of the operand itself.
func PrintOperand(op Operand) string {
	return op.String()
}

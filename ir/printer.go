package ir

import (
	"bytes"
	"fmt"
	"strings"
)

// Printer holds the state for printing IR AST nodes.
type Printer struct {
	buf         bytes.Buffer
	indentation int
}

// NewPrinter creates a new Printer.
func NewPrinter() *Printer {
	return &Printer{}
}

// PrintFile converts an entire IR File AST to its string representation.
func (p *Printer) PrintFile(file *File) string {
	p.buf.Reset()
	for i, node := range file.Nodes {
		if i > 0 {
			p.buf.WriteString("\n\n") // Add a blank line between top-level nodes
		}
		p.printNode(node)
	}
	return p.buf.String()
}

func (p *Printer) printNode(node Node) {
	switch n := node.(type) {
	case *TypeDefinition:
		p.printTypeDefinition(n)
	case *FunctionDefinition:
		p.printFunctionDefinition(n)
	default:
		p.buf.WriteString(fmt.Sprintf("<?unknown node type %T>", n))
	}
}

func (p *Printer) printTypeDefinition(td *TypeDefinition) {
	p.buf.WriteString("type ")
	p.buf.WriteString(td.Name)
	p.buf.WriteString(" ")
	p.printType(td.Type)
}

func (p *Printer) printType(t Type) {
	switch ty := t.(type) {
	case *BasicType:
		p.buf.WriteString(ty.Name)
	case *PointerType:
		p.buf.WriteString("ptr<")
		p.printType(ty.ElementType)
		p.buf.WriteString(">")
	case *StructType:
		p.buf.WriteString("{")
		for i, fieldType := range ty.Fields {
			if i > 0 {
				p.buf.WriteString(", ")
			}
			p.printType(fieldType)
		}
		p.buf.WriteString("}")
	default:
		p.buf.WriteString(fmt.Sprintf("<?unknown type %T>", ty))
	}
}

func (p *Printer) printFunctionDefinition(fn *FunctionDefinition) {
	p.buf.WriteString("fn ")
	p.buf.WriteString(fn.Name)
	p.buf.WriteString("(")
	for i, param := range fn.Parameters {
		if i > 0 {
			p.buf.WriteString(", ")
		}
		p.buf.WriteString(param.Name)
		p.buf.WriteString(" : ")
		p.printType(param.Type)
	}
	p.buf.WriteString(") : ")
	p.printType(fn.ReturnType)
	p.buf.WriteString(" {\n")

	labelPositions := make(map[int][]string)
	for labelName, index := range fn.Labels {
		labelPositions[index] = append(labelPositions[index], labelName)
	}

	p.indentation++
	for i, instruction := range fn.Body {
		if labels, ok := labelPositions[i]; ok {
			p.indentation--
			for _, labelName := range labels {
				p.writeIndent()
				p.buf.WriteString(labelName)
				p.buf.WriteString(":\n")
			}
			p.indentation++
		}
		p.writeIndent()
		p.printInstruction(instruction)
		p.buf.WriteString("\n")
	}
	p.indentation--
	p.buf.WriteString("}")
}

func (p *Printer) writeIndent() {
	p.buf.WriteString(strings.Repeat("  ", p.indentation)) // 2 spaces for indentation
}

func (p *Printer) printInstruction(instr Instruction) {
	switch i := instr.(type) {
	case *ConstantAssignmentInstruction:
		p.buf.WriteString(i.VarName)
		p.buf.WriteString(" = ")
		p.buf.WriteString(i.Value)
		p.buf.WriteString(" : ")
		p.printType(i.Type)
	case *BinaryExpressionInstruction:
		p.buf.WriteString(i.ResultVar)
		p.buf.WriteString(" = ")
		p.buf.WriteString(i.Operand1)
		p.buf.WriteString(" ")
		p.buf.WriteString(i.Operator)
		p.buf.WriteString(" ")
		p.buf.WriteString(i.Operand2)
		p.buf.WriteString(" : ")
		p.printType(i.Type)
	case *AllocInstruction:
		p.buf.WriteString(i.VarName)
		p.buf.WriteString(" = alloc ")
		p.buf.WriteString(i.TypeName)
		p.buf.WriteString(" : ")
		p.printType(i.ReturnType)
	case *LoadInstruction:
		p.buf.WriteString(i.VarName)
		p.buf.WriteString(" = load ")
		p.buf.WriteString(i.Pointer)
		if i.Index != nil {
			p.buf.WriteString(", ")
			p.buf.WriteString(fmt.Sprintf("%d", *i.Index))
		}
	case *StoreInstruction:
		p.buf.WriteString("store ")
		p.buf.WriteString(i.Pointer)
		p.buf.WriteString(", ")
		p.buf.WriteString(i.Value)
		if i.Index != nil {
			p.buf.WriteString(", ")
			p.buf.WriteString(fmt.Sprintf("%d", *i.Index))
		}
	case *CallInstruction:
		p.buf.WriteString(i.VarName)
		p.buf.WriteString(" = call ")
		p.buf.WriteString(i.FunctionName)
		p.buf.WriteString("(")
		for idx, arg := range i.Arguments {
			if idx > 0 {
				p.buf.WriteString(", ")
			}
			p.buf.WriteString(arg)
		}
		p.buf.WriteString(")")
		p.buf.WriteString(" : ")
		p.printType(i.ReturnType)
	case *ReturnInstruction:
		p.buf.WriteString("return ")
		p.buf.WriteString(i.Value)
	case *BranchInstruction:
		p.buf.WriteString("br ")
		p.buf.WriteString(i.Label)
	case *ConditionalBranchInstruction:
		p.buf.WriteString("br_cond ")
		p.buf.WriteString(i.Condition)
		p.buf.WriteString(", ")
		p.buf.WriteString(i.TrueLabel)
		p.buf.WriteString(", ")
		p.buf.WriteString(i.FalseLabel)
	case *FreeInstruction:
		p.buf.WriteString("free ")
		p.buf.WriteString(i.Value)
	default:
		p.buf.WriteString(fmt.Sprintf("<?unknown instruction type %T>", i))
	}
}

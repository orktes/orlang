package ir

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/orktes/orlang/ast"      // For ast.Position
	"github.com/orktes/orlang/scanner" // Corrected import
)

// Parser holds the state of the parser.
type Parser struct {
	s           *Scanner // Custom IR scanner
	tok         scanner.Token
	tokenBuffer []scanner.Token // For unreading/peeking tokens
	lastTokens  []scanner.Token // History for complex unread, though simple peek might not need full history

	currentProgram *Program
	currentFunc    *FunctionDefinition
	localVars      map[string]Variable // Scope for current function's variables
}

// NewParser creates a new parser.
func NewParser(s *Scanner) *Parser {
	p := &Parser{
		s:           s,
		localVars:   make(map[string]Variable),
		tokenBuffer: make([]scanner.Token, 0, 2), // Initial capacity
		lastTokens:  make([]scanner.Token, 0, 1), // Store the single last "advanced" token
	}
	p.advance() // Initialize first token into p.tok
	return p
}

// read directly fetches the next token from the scanner.
// It's a low-level method. `advance` is the public way to move to the next token.
func (p *Parser) readFromScanner() scanner.Token {
	// In a more complex system, this might skip comments/whitespace.
	// Here, we assume ir.Scanner provides only relevant tokens.
	return p.s.Scan()
}

// advance consumes the current p.tok and reads the next significant token into p.tok.
func (p *Parser) advance() {
	// Store the token that *was* current p.tok (if it's valid) into lastTokens.
	// This is for potential complex unread scenarios or debugging.
	// For simple peek, a single last token might be enough.
	if p.tok.Type != scanner.TokenTypeUndefined { // Avoid storing initial undefined token
		// lastTokens will store just the immediate last token.
		if len(p.lastTokens) > 0 {
			p.lastTokens = p.lastTokens[:0]
		}
		p.lastTokens = append(p.lastTokens, p.tok)
	}

	if len(p.tokenBuffer) > 0 {
		p.tok = p.tokenBuffer[len(p.tokenBuffer)-1]
		p.tokenBuffer = p.tokenBuffer[:len(p.tokenBuffer)-1]
	} else {
		p.tok = p.readFromScanner()
	}
}

// unreadCurrent pushes the current token p.tok back to the buffer.
// After this, p.tok is typically restored to the token before the one pushed back.
// This is primarily for use by peek().
func (p *Parser) unreadCurrent() {
    if p.tok.Type != scanner.TokenTypeUndefined { // Do not unread an undefined token
        p.tokenBuffer = append(p.tokenBuffer, p.tok)
    }
    // Restore p.tok to the actual previous token if available in lastTokens
    if len(p.lastTokens) > 0 {
        p.tok = p.lastTokens[len(p.lastTokens)-1]
        // p.lastTokens = p.lastTokens[:len(p.lastTokens)-1] // Pop, so next unreadCurrent would get token before that
                                                          // Or, keep lastTokens as the token *before* the current p.tok always.
                                                          // Let's keep it simple: lastTokens[0] is the token before current p.tok.
    } else {
        // If no lastTokens, p.tok becomes undefined or needs special handling.
        // This means we unread the very first token that was advanced into p.tok.
        p.tok = scanner.Token{Type: scanner.TokenTypeUndefined}
    }
}


// peek looks at the next token from the scanner without "consuming" it from the parser's main view.
func (p *Parser) peek() scanner.Token {
	// 1. Remember current token
	originalTok := p.tok

	// 2. Advance to get the next token into p.tok
	p.advance()
	peekedTok := p.tok

	// 3. "Unread" the peeked token by pushing it to the buffer
	p.tokenBuffer = append(p.tokenBuffer, peekedTok)

	// 4. Restore the parser's current token to the original one
	p.tok = originalTok

	return peekedTok
}


func (p *Parser) error(msg string) {
	panic(fmt.Sprintf("Parser error at %s (token type %s, text '%s'): %s", p.tok.Position, p.tok.Type, p.tok.Text, msg))
}

func (p *Parser) expect(expectedType scanner.TokenType) string {
	if p.tok.Type == expectedType {
		text := p.tok.Text
		p.advance()
		return text
	}
	p.error(fmt.Sprintf("expected token type %s, got %s (text '%s')", expectedType, p.tok.Type, p.tok.Text))
	return "" // Unreachable
}

func (p *Parser) expectKeyword(keyword string) {
	if p.tok.Type == scanner.TokenTypeIdent && p.tok.Text == keyword {
		p.advance()
	} else {
		p.error(fmt.Sprintf("expected keyword '%s', got '%s' (type %s)", keyword, p.tok.Text, p.tok.Type))
	}
}

func (p *Parser) Parse() *Program {
	p.currentProgram = &Program{
		Structs:   make(map[string]*StructTypeDefinition),
		Functions: make(map[string]*FunctionDefinition),
	}
	// p.advance() already called by constructor

	for p.tok.Type != scanner.TokenTypeEOF {
		if p.tok.Type != scanner.TokenTypeIdent {
			p.error(fmt.Sprintf("expected 'type' or 'fn' keyword at top level, got %s (text '%s')", p.tok.Type, p.tok.Text))
		}
		switch p.tok.Text {
		case "type":
			p.parseStructTypeDefinition()
		case "fn":
			p.parseFunctionDefinition()
		default:
			p.error(fmt.Sprintf("unexpected identifier '%s' at top level", p.tok.Text))
		}
	}
	return p.currentProgram
}

func (p *Parser) parseType() Type {
	if p.tok.Type == scanner.TokenTypeIdent && p.tok.Text == "ptr" {
		p.advance() // consume "ptr"
		p.expect(scanner.TokenTypeLT)
		elementType := p.parseType()
		p.expect(scanner.TokenTypeGT)
		return &PointerType{Element: elementType}
	} else if p.tok.Type == scanner.TokenTypeIdent {
		typeName := p.tok.Text
		p.advance() // consume type name (IDENT)
		if st, ok := p.currentProgram.Structs[typeName]; ok {
			return st
		}
		return &SimpleType{Name: typeName}
	}
	p.error(fmt.Sprintf("unexpected token %s (text '%s') when parsing type", p.tok.Type, p.tok.Text))
	return nil
}

func (p *Parser) parseStructTypeDefinition() {
	p.expectKeyword("type")
	name := p.expect(scanner.TokenTypeIdent)
	p.expect(scanner.TokenTypeLBrace)

	var fields []Type
	if p.tok.Type != scanner.TokenTypeRBrace {
		fields = append(fields, p.parseType())
		for p.tok.Type == scanner.TokenTypeComma {
			p.advance() // consume ","
			if p.tok.Type == scanner.TokenTypeRBrace {
				break
			}
			fields = append(fields, p.parseType())
		}
	}
	p.expect(scanner.TokenTypeRBrace)

	structDef := &StructTypeDefinition{Name: name, Fields: fields}
	if _, exists := p.currentProgram.Structs[name]; exists {
		p.error(fmt.Sprintf("struct type '%s' redefined", name))
	}
	p.currentProgram.Structs[name] = structDef
}

func (p *Parser) parseFunctionDefinition() {
	p.expectKeyword("fn")
	name := p.expect(scanner.TokenTypeIdent)

	p.currentFunc = &FunctionDefinition{Name: name, Parent: p.currentProgram}
	p.localVars = make(map[string]Variable)

	p.expect(scanner.TokenTypeLParen)
	var params []Variable
	if p.tok.Type != scanner.TokenTypeRParen {
		params = p.parseParameterList()
	}
	p.expect(scanner.TokenTypeRParen)
	p.currentFunc.Parameters = params
	for _, param := range params {
		if _, exists := p.localVars[param.Name]; exists {
			p.error(fmt.Sprintf("parameter '%%%s' redefined in function '%s'", param.Name, name))
		}
		p.localVars[param.Name] = param
	}

	p.expect(scanner.TokenTypeColon)
	p.currentFunc.ReturnType = p.parseType()

	p.expect(scanner.TokenTypeLBrace)
	p.currentFunc.Body = p.parseInstructionList()
	p.expect(scanner.TokenTypeRBrace)

	if _, exists := p.currentProgram.Functions[name]; exists {
		p.error(fmt.Sprintf("function '%s' redefined", name))
	}
	p.currentProgram.Functions[name] = p.currentFunc
	p.currentFunc = nil
}

func (p *Parser) parseParameterList() []Variable {
	var params []Variable
	params = append(params, p.parseParameter())
	for p.tok.Type == scanner.TokenTypeComma {
		p.advance() // consume ","
		params = append(params, p.parseParameter())
	}
	return params
}

func (p *Parser) parseParameter() Variable {
	nameTokenText := p.tok.Text
	// Expect IDENT starting with %
	if !(p.tok.Type == scanner.TokenTypeIdent && strings.HasPrefix(nameTokenText, "%")) {
		p.error(fmt.Sprintf("parameter name '%s' must be an IDENT starting with %%", nameTokenText))
	}
	p.advance() // consume "%name"

	name := strings.TrimPrefix(nameTokenText, "%")
	p.expect(scanner.TokenTypeColon)
	typ := p.parseType()
	return Variable{Name: name, Type: typ}
}

func (p *Parser) parseOperand() Operand {
	if p.tok.Type == scanner.TokenTypeIdent && strings.HasPrefix(p.tok.Text, "%") {
		name := strings.TrimPrefix(p.tok.Text, "%")
		p.advance() // consume variable token ("%name")
		if v, ok := p.localVars[name]; ok {
			return &v
		}
		p.error(fmt.Sprintf("undefined variable '%%%s'", name))
	} else if p.tok.Type == scanner.TokenTypeInteger || p.tok.Type == scanner.TokenTypeFloat ||
		(p.tok.Type == scanner.TokenTypeIdent && (p.tok.Text == "true" || p.tok.Text == "false")) {
		val := p.tok.Text
		tokType := p.tok.Type
		p.advance() // consume literal

		var typ Type
		switch tokType {
		case scanner.TokenTypeInteger:
			typ = &SimpleType{Name: "int_const_default"}
		case scanner.TokenTypeFloat:
			typ = &SimpleType{Name: "float_const_default"}
		case scanner.TokenTypeIdent: // true or false
			typ = &SimpleType{Name: "bool"}
		}

		if p.tok.Type == scanner.TokenTypeColon {
			p.advance() // consume ":"
			typ = p.parseType()
		}
		return &Constant{Value: val, Type: typ}
	}
	p.error(fmt.Sprintf("unexpected token %s (text '%s') when parsing operand", p.tok.Type, p.tok.Text))
	return nil
}

func (p *Parser) parseInstructionList() []Instruction {
	var instructions []Instruction
	for p.tok.Type != scanner.TokenTypeRBrace && p.tok.Type != scanner.TokenTypeEOF {
		instructions = append(instructions, p.parseInstruction())
	}
	return instructions
}

func (p *Parser) parseInstruction() Instruction {
	// Label: IDENT (text like "labelName:")
	if p.tok.Type == scanner.TokenTypeIdent && strings.HasSuffix(p.tok.Text, ":") {
		if strings.HasPrefix(p.tok.Text, "%") {
			// Fall through, might be an assignment like %var: ... which is an error,
			// but will be caught by assignment parsing.
		} else {
			labelName := strings.TrimSuffix(p.tok.Text, ":")
			p.advance() // consume "labelName:" token
			return &Label{Name: labelName}
		}
	}

	// Assignment: %dest_var_name = ...
	if p.tok.Type == scanner.TokenTypeIdent && strings.HasPrefix(p.tok.Text, "%") {
		destNameTokenText := p.tok.Text
		p.advance() // consume "%dest_var_name"

		if p.tok.Type != scanner.TokenTypeAssign {
			p.error(fmt.Sprintf("expected '=' after variable '%s' for assignment, got %s (text '%s')", destNameTokenText, p.tok.Type, p.tok.Text))
		}
		p.expect(scanner.TokenTypeAssign) // consume "=" (advances)

		return p.parseAssignmentRHS(strings.TrimPrefix(destNameTokenText, "%"))
	}

	// Keyword-based instructions
	if p.tok.Type == scanner.TokenTypeIdent {
		keyword := p.tok.Text
		p.advance() // Consume keyword's IDENT token

		switch keyword {
		case "return":
			return p.parseReturnInstruction()
		case "store":
			return p.parseStoreInstruction()
		case "free":
			return p.parseFreeInstruction()
		case "br":
			return p.parseBranchInstruction()
		case "br_cond":
			return p.parseConditionalBranchInstruction()
		default:
			p.error(fmt.Sprintf("unexpected instruction keyword '%s'", keyword))
		}
	}

	p.error(fmt.Sprintf("unexpected token %s (text '%s') at start of instruction", p.tok.Type, p.tok.Text))
	return nil
}

func (p *Parser) parseAssignmentRHS(destName string) Instruction {
	if p.tok.Type != scanner.TokenTypeIdent {
		p.error(fmt.Sprintf("expected 'alloc', 'load', or 'call' after '=', got %s (text '%s')", p.tok.Type, p.tok.Text))
	}
	keyword := p.tok.Text
	p.advance() // consume keyword (alloc, load, call)

	switch keyword {
	case "alloc":
		allocType := p.parseType() // advances past type
		destVar := Variable{Name: destName, Type: &PointerType{Element: allocType}}
		p.localVars[destName] = destVar
		return &AllocInstruction{Type: allocType, Dest: destVar}

	case "load":
		ptrOperand := p.parseOperand() // advances
		var indexOperand Operand
		if p.tok.Type == scanner.TokenTypeComma {
			p.advance() // consume ","
			indexOperand = p.parseOperand() // advances
		}

		var destType Type
		ptrVar, ok := ptrOperand.(*Variable)
		if !ok {
			p.error(fmt.Sprintf("load source '%s' must be a variable", ptrOperand.String()))
		}
		pointerType, okType := ptrVar.Type.(*PointerType)
		if !okType {
			p.error(fmt.Sprintf("load source variable '%%%s' (type %s) is not a pointer type", ptrVar.Name, ptrVar.Type.String()))
		}

		if indexOperand == nil {
			destType = pointerType.Element
		} else {
			structType, okStruct := pointerType.Element.(*StructTypeDefinition)
			if !okStruct {
				p.error(fmt.Sprintf("cannot use index on non-struct pointer type %s", pointerType.String()))
			}
			idxConst, okConst := indexOperand.(*Constant)
			if !okConst {
				p.error(fmt.Sprintf("struct field index '%s' must be an integer constant", indexOperand.String()))
			}
			idx, err := strconv.Atoi(idxConst.Value)
			if err != nil || idx < 0 || idx >= len(structType.Fields) {
				p.error(fmt.Sprintf("invalid struct field index '%s' for struct '%s'", idxConst.Value, structType.Name))
			}
			destType = structType.Fields[idx]
		}
		destVar := Variable{Name: destName, Type: destType}
		p.localVars[destName] = destVar
		return &LoadInstruction{Dest: destVar, Pointer: ptrOperand, Index: indexOperand}

	case "call":
		funcNameText := p.expect(scanner.TokenTypeIdent) // advances
		p.expect(scanner.TokenTypeLParen) // advances
		var args []Operand
		if p.tok.Type != scanner.TokenTypeRParen {
			args = append(args, p.parseOperand()) // advances
			for p.tok.Type == scanner.TokenTypeComma {
				p.advance() // consume ","
				args = append(args, p.parseOperand()) // advances
			}
		}
		p.expect(scanner.TokenTypeRParen) // advances
		p.expect(scanner.TokenTypeColon)  // advances
		returnType := p.parseType()       // advances

		destVar := Variable{Name: destName, Type: returnType}
		p.localVars[destName] = destVar
		return &CallInstruction{Target: funcNameText, Arguments: args, Dest: destVar}
	}
	p.error(fmt.Sprintf("unexpected keyword '%s' on RHS of assignment", keyword))
	return nil
}

func (p *Parser) parseReturnInstruction() *ReturnInstruction {
	val := p.parseOperand() // advances
	return &ReturnInstruction{Value: val}
}

func (p *Parser) parseStoreInstruction() *StoreInstruction {
	ptr := p.parseOperand()           // advances
	p.expect(scanner.TokenTypeComma) // advances
	val := p.parseOperand()           // advances
	var index Operand
	if p.tok.Type == scanner.TokenTypeComma {
		p.advance() // consume ","
		index = p.parseOperand() // advances
	}
	return &StoreInstruction{Pointer: ptr, Value: val, Index: index}
}

func (p *Parser) parseFreeInstruction() *FreeInstruction {
	ptr := p.parseOperand() // advances
	return &FreeInstruction{Pointer: ptr}
}

func (p *Parser) parseBranchInstruction() *BranchInstruction {
	label := p.expect(scanner.TokenTypeIdent) // advances
	return &BranchInstruction{Label: label}
}

func (p *Parser) parseConditionalBranchInstruction() *ConditionalBranchInstruction {
	cond := p.parseOperand()          // advances
	p.expect(scanner.TokenTypeComma) // advances
	trueLabel := p.expect(scanner.TokenTypeIdent) // advances
	p.expect(scanner.TokenTypeComma) // advances
	falseLabel := p.expect(scanner.TokenTypeIdent) // advances
	return &ConditionalBranchInstruction{Condition: cond, TrueLabel: trueLabel, FalseLabel: falseLabel}
}

var _ ast.Position // Keep ast import
```

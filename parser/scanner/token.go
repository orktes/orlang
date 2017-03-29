package scanner

import "fmt"

// TokenType is an int value representing the token type
type TokenType int

const (
	// TokenTypeUnknown unknown token
	TokenTypeUnknown TokenType = iota
	// TokenTypeEOF represents end of input data
	TokenTypeEOF
	// TokenTypeIdent ident
	TokenTypeIdent
	// TokenTypeMacroIdent ident used in macros
	TokenTypeMacroIdent
	// TokenTypeMacroCallIdent ident used in macro calls
	TokenTypeMacroCallIdent
	// TokenTypeWhitespace whitespace
	TokenTypeWhitespace
	// TokenTypeString string literal
	TokenTypeString
	// TokenTypeNumber number/integer
	TokenTypeNumber
	// TokenTypeFloat float
	TokenTypeFloat
	// TokenTypeBoolean boolean
	TokenTypeBoolean
	// TokenTypeComment comment block
	TokenTypeComment
	// TokenTypeLess left chevron <
	TokenTypeLess
	// TokenTypeLPAREN left parenthesis (
	TokenTypeLPAREN
	// TokenTypeLBRACK left bracket [
	TokenTypeLBRACK
	// TokenTypeLBRACE left brace {
	TokenTypeLBRACE
	// TokenTypeGreater left chevron >
	TokenTypeGreater
	// TokenTypeRPAREN right parenthesis )
	TokenTypeRPAREN
	// TokenTypeRBRACK left bracket [
	TokenTypeRBRACK
	// TokenTypeRBRACE right brace {
	TokenTypeRBRACE
	// TokenTypeCOMMA comma ,
	TokenTypeCOMMA
	// TokenTypePERIOD period .
	TokenTypePERIOD
	// TokenTypeCOLON colon :
	TokenTypeCOLON
	// TokenTypeSEMICOLON semicolon ;
	TokenTypeSEMICOLON
	// TokenTypeASSIGN assigment/equals =
	TokenTypeASSIGN
	// TokenTypeADD addition/plus sign +
	TokenTypeADD
	// TokenTypeSUB subtraction/minux sign -
	TokenTypeSUB
	// TokenTypeASTERIX asterix/pointer/times
	TokenTypeASTERIX
	// TokenTypeAMPERSAND ampersan &
	TokenTypeAMPERSAND
	// TokenTypeDOLLAR ampersan $
	TokenTypeDOLLAR
	// TokenTypeHASHBANG hashbang #
	TokenTypeHASHBANG
	// TokenTypeEXCL exclamation mark
	TokenTypeEXCL
	// TokenTypeSLASH slash
	TokenTypeSLASH
	// TokenTypeBACKSLASH backslash
	TokenTypeBACKSLASH

	// TokenTypeEqual ==
	TokenTypeEqual
	// TokenTypeNotEqual !=
	TokenTypeNotEqual
	// TokenTypeLessOrEqual <=
	TokenTypeLessOrEqual
	// TokenTypeGreaterOrEqual >=
	TokenTypeGreaterOrEqual

	// TokenTypeIncrement ++
	TokenTypeIncrement
	// TokenTypeDecrement --
	TokenTypeDecrement

	// TokenTypeEllipsis ...
	TokenTypeEllipsis
)

var tokenNames = [...]string{
	TokenTypeUnknown: "UNKNOWN",

	TokenTypeEOF:     "EOF",
	TokenTypeComment: "COMMENT",

	TokenTypeIdent:          "IDENT",
	TokenTypeMacroIdent:     "MACROIDENT",
	TokenTypeMacroCallIdent: "MACROCALLIDENT",
	TokenTypeNumber:         "NUMBER",
	TokenTypeFloat:          "FLOAT",
	TokenTypeBoolean:        "BOOL",
	TokenTypeString:         "STRING",

	TokenTypeLBRACK: "LBRACK",
	TokenTypeLBRACE: "LBRACE",
	TokenTypeLPAREN: "LPAREN",
	TokenTypeLess:   "LCHEV",

	TokenTypeRBRACK:  "RBRACK",
	TokenTypeRBRACE:  "RBRACE",
	TokenTypeRPAREN:  "RPAREN",
	TokenTypeGreater: "RCHEV",

	TokenTypeCOMMA:     "COMMA",
	TokenTypePERIOD:    "PERIOD",
	TokenTypeCOLON:     "COLON",
	TokenTypeSEMICOLON: "SEMICOLON",

	TokenTypeASSIGN:     "ASSIGN",
	TokenTypeADD:        "ADD",
	TokenTypeSUB:        "SUB",
	TokenTypeAMPERSAND:  "AMPERSAND",
	TokenTypeASTERIX:    "ASTERIX",
	TokenTypeDOLLAR:     "DOLLAR",
	TokenTypeWhitespace: "WHITESPACE",
	TokenTypeHASHBANG:   "HASHBANG",
	TokenTypeEXCL:       "EXCLAMATION",

	TokenTypeSLASH:     "SLASH",
	TokenTypeBACKSLASH: "BACKSLASH",

	TokenTypeEqual:          "EQUAL",
	TokenTypeNotEqual:       "NOTEQUAL",
	TokenTypeLessOrEqual:    "LESSOREQUAL",
	TokenTypeGreaterOrEqual: "GREATEROREQUAL",

	TokenTypeIncrement: "INCREMENT",
	TokenTypeDecrement: "DECREMENT",

	TokenTypeEllipsis: "ELLIPSIS",
}

func (typ TokenType) String() string {
	return tokenNames[typ]
}

// Token holds type, position and literal info of a token
type Token struct {
	Text        string
	Value       interface{}
	Type        TokenType
	StartLine   int
	StartColumn int
	EndLine     int
	EndColumn   int
}

func (t Token) StringValue() string {
	if t.Type >= TokenTypeEqual {
		return t.Text
	}

	if t.Value == nil || t.Type == TokenTypeUnknown || t.Type == TokenTypeIdent {
		return fmt.Sprintf("%s(%s)", t.Type.String(), t.Text)
	}

	return fmt.Sprintf("%s(%v)", t.Type.String(), t.Value)
}

func (t Token) String() string {
	return fmt.Sprintf("%d:%d %s", t.StartLine, t.StartColumn, t.StringValue())
}

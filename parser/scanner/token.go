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
	// TokenTypeLCHEV left chevron <
	TokenTypeLCHEV
	// TokenTypeLPAREN left parenthesis (
	TokenTypeLPAREN
	// TokenTypeLBRACK left bracket [
	TokenTypeLBRACK
	// TokenTypeLBRACE left brace {
	TokenTypeLBRACE
	// TokenTypeRCHEV left chevron >
	TokenTypeRCHEV
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
	// TokenTypeHASHBANG hashbang #
	TokenTypeHASHBANG
	// TokenTypeEXCL exclamation mark
	TokenTypeEXCL
	// TokenTypeSLASH slash
	TokenTypeSLASH
	// TokenTypeBACKSLASH backslash
	TokenTypeBACKSLASH
)

var tokenNames = [...]string{
	TokenTypeUnknown: "UNKNOWN",

	TokenTypeEOF:     "EOF",
	TokenTypeComment: "COMMENT",

	TokenTypeIdent:   "IDENT",
	TokenTypeNumber:  "NUMBER",
	TokenTypeFloat:   "FLOAT",
	TokenTypeBoolean: "BOOL",
	TokenTypeString:  "STRING",

	TokenTypeLBRACK: "LBRACK",
	TokenTypeLBRACE: "LBRACE",
	TokenTypeLPAREN: "LPAREN",
	TokenTypeLCHEV:  "LCHEV",

	TokenTypeRBRACK: "RBRACK",
	TokenTypeRBRACE: "RBRACE",
	TokenTypeRPAREN: "RPAREN",
	TokenTypeRCHEV:  "RCHEV",

	TokenTypeCOMMA:     "COMMA",
	TokenTypePERIOD:    "PERIOD",
	TokenTypeCOLON:     "COLON",
	TokenTypeSEMICOLON: "SEMICOLON",

	TokenTypeASSIGN:     "ASSIGN",
	TokenTypeADD:        "ADD",
	TokenTypeSUB:        "SUB",
	TokenTypeAMPERSAND:  "AMPERSAND",
	TokenTypeASTERIX:    "ASTERIX",
	TokenTypeWhitespace: "WHITESPACE",
	TokenTypeHASHBANG:   "HASHBANG",
	TokenTypeEXCL:       "EXCLAMATION",

	TokenTypeSLASH:     "SLASH",
	TokenTypeBACKSLASH: "BACKSLASH",
}

func (typ TokenType) String() string {
	return tokenNames[typ]
}

// Token holds type, position and literal info of a token
type Token struct {
	Text   string
	Value  interface{}
	Type   TokenType
	Line   int
	Column int
}

func (t Token) String() string {
	if t.Value == nil || t.Type == TokenTypeUnknown {
		return fmt.Sprintf("%d:%d %s(%s)", t.Line, t.Column, t.Type.String(), t.Text)
	}

	return fmt.Sprintf("%d:%d %s(%v)", t.Line, t.Column, t.Type.String(), t.Value)
}

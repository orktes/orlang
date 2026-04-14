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
	// TokenTypeASTERISK asterisk/pointer/times
	TokenTypeASTERISK
	// TokenTypeAMPERSAND ampersan &
	TokenTypeAMPERSAND
	// TokenTypeDOLLAR ampersan $
	TokenTypeDOLLAR
	// TokenTypeHASHBANG hashbang #
	TokenTypeHASHBANG
	// TokenTypeEXCL exclamation mark
	TokenTypeEXCL
	// TokenTypeQUESTIONMARK ?
	TokenTypeQUESTIONMARK
	// TokenTypeSLASH slash
	TokenTypeSLASH
	// TokenTypeBACKSLASH backslash
	TokenTypeBACKSLASH
	// TokenTypePERCENT percent/modulo %
	TokenTypePERCENT
	// TokenTypePIPE bitwise OR |
	TokenTypePIPE
	// TokenTypeCARET bitwise XOR ^
	TokenTypeCARET
	// TokenTypeLSHIFT left shift <<
	TokenTypeLSHIFT
	// TokenTypeRSHIFT right shift >>
	TokenTypeRSHIFT
	// TokenTypeTILDE bitwise NOT ~
	TokenTypeTILDE

	// TokenTypeEqual ==
	TokenTypeEqual
	// TokenTypeNotEqual !=
	TokenTypeNotEqual
	// TokenTypeLessOrEqual <=
	TokenTypeLessOrEqual
	// TokenTypeGreaterOrEqual >=
	TokenTypeGreaterOrEqual

	// TokenTypeIs is (type assertion)
	TokenTypeIs

	// TokenTypeOr logical OR ||
	TokenTypeOr
	// TokenTypeAnd logical AND &&
	TokenTypeAnd

	// TokenTypeAddAssign +=
	TokenTypeAddAssign
	// TokenTypeSubAssign -=
	TokenTypeSubAssign
	// TokenTypeMulAssign *=
	TokenTypeMulAssign
	// TokenTypeDivAssign /=
	TokenTypeDivAssign
	// TokenTypeModAssign %=
	TokenTypeModAssign

	// TokenTypeIncrement ++
	TokenTypeIncrement
	// TokenTypeDecrement --
	TokenTypeDecrement

	// TokenTypeEllipsis ...
	TokenTypeEllipsis

	// TokenTypeArrow =>
	TokenTypeArrow

	// TokenTypeImport import
	TokenTypeImport
	// TokenTypeExport export
	TokenTypeExport
	// TokenTypeFrom from
	TokenTypeFrom
	// TokenTypeInclude include
	TokenTypeInclude
	// TokenTypeAs as
	TokenTypeAs
	// TokenTypeBreak break
	TokenTypeBreak
	// TokenTypeContinue continue
	TokenTypeContinue
	// TokenTypeSwitch switch
	TokenTypeSwitch
	// TokenTypeCase case
	TokenTypeCase
	// TokenTypeDefault default
	TokenTypeDefault
	// TokenTypeDefer defer
	TokenTypeDefer
	// TokenTypeLink link
	TokenTypeLink
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

	TokenTypeASSIGN: "ASSIGN",

	TokenTypeADD:          "ADD",
	TokenTypeSUB:          "SUB",
	TokenTypeAMPERSAND:    "AMPERSAND",
	TokenTypeASTERISK:     "ASTERISK",
	TokenTypeDOLLAR:       "DOLLAR",
	TokenTypeWhitespace:   "WHITESPACE",
	TokenTypeHASHBANG:     "HASHBANG",
	TokenTypeEXCL:         "EXCLAMATION",
	TokenTypeQUESTIONMARK: "QUESTIONMARK",

	TokenTypeSLASH:     "SLASH",
	TokenTypeBACKSLASH: "BACKSLASH",
	TokenTypePERCENT:   "PERCENT",
	TokenTypePIPE:      "PIPE",
	TokenTypeCARET:     "CARET",
	TokenTypeLSHIFT:    "LSHIFT",
	TokenTypeRSHIFT:    "RSHIFT",
	TokenTypeTILDE:     "TILDE",

	TokenTypeEqual:          "EQUAL",
	TokenTypeNotEqual:       "NOTEQUAL",
	TokenTypeLessOrEqual:    "LESSOREQUAL",
	TokenTypeGreaterOrEqual: "GREATEROREQUAL",
	TokenTypeIs:             "IS",
	TokenTypeOr:             "OR",
	TokenTypeAnd:            "AND",

	TokenTypeAddAssign: "ADDASSIGN",
	TokenTypeSubAssign: "SUBASSIGN",
	TokenTypeMulAssign: "MULASSIGN",
	TokenTypeDivAssign: "DIVASSIGN",
	TokenTypeModAssign: "MODASSIGN",

	TokenTypeIncrement: "INCREMENT",
	TokenTypeDecrement: "DECREMENT",

	TokenTypeEllipsis: "ELLIPSIS",
	TokenTypeArrow:    "ARROW",
	TokenTypeImport:   "IMPORT",
	TokenTypeExport:   "EXPORT",
	TokenTypeFrom:     "FROM",
	TokenTypeInclude:  "INCLUDE",
	TokenTypeAs:       "AS",
	TokenTypeBreak:    "BREAK",
	TokenTypeContinue: "CONTINUE",
	TokenTypeSwitch:   "SWITCH",
	TokenTypeCase:     "CASE",
	TokenTypeDefault:  "DEFAULT",
	TokenTypeDefer:    "DEFER",
	TokenTypeLink:     "LINK",
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

	if t.Text == "" {
		return t.Type.String()
	}

	if t.Value == nil || t.Type == TokenTypeUnknown || t.Type == TokenTypeIdent {
		return fmt.Sprintf("%s(%s)", t.Type.String(), t.Text)
	}

	return fmt.Sprintf("%s(%v)", t.Type.String(), t.Value)
}

func (t Token) String() string {
	return fmt.Sprintf("%d:%d %s", t.StartLine, t.StartColumn, t.StringValue())
}

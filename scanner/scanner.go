package scanner

import (
	"bufio"
	"bytes"
	"io"
	"math"
	"strconv"
)

// TokenChannelSize how many tokens can be buffered into the scan channel (default to 10)
var TokenChannelSize = 10

const eof = rune(0)

// Scanner scans given io.Reader for tokens
type Scanner struct {
	r       *bufio.Reader
	lastPos struct {
		line   int
		column int
	}
	pos struct {
		line   int
		column int
	}
	Error func(msg string)
}

// NewScanner returns a new scanner for io.Reader
func NewScanner(r io.Reader) *Scanner {
	return &Scanner{r: bufio.NewReader(r)}
}

// ScanChannel return read only channel for tokens (closes on EOF)
func (s *Scanner) ScanChannel() (token <-chan Token) {
	c := make(chan Token, TokenChannelSize)
	go func(c chan<- Token) {
		for {
			token := s.Scan()
			c <- token
			if token.Type == TokenTypeEOF {
				break
			}

		}
		close(c)
	}(c)

	return c
}

// Scan returns next token
func (s *Scanner) Scan() (token Token) {
	token.StartColumn = s.pos.column
	token.StartLine = s.pos.line

	ch := s.read()

	var t TokenType
	var text string
	var val interface{}

	switch {
	case isWhitespace(ch):
		s.unread()
		t, text = s.scanWhitespace()

	case isLetter(ch):
		s.unread()
		t, text, val = s.scanIdent()
		if t == TokenTypeIdent {
			next := s.read()
			if next == '!' {
				t = TokenTypeMacroCallIdent
				val = text
				text = text + "!"
			} else {
				s.unread()
			}
		}

	case ch == '$':
		t = TokenTypeDOLLAR
		next := s.peek()
		if isLetter(next) {
			t = TokenTypeMacroIdent
			_, text, val = s.scanIdent()
			text = "$" + text
		} else {
			text = "$"
		}

	case isNumber(ch):
		t, text, val = s.scanNumber(ch)

	case ch == '/':
		s.unread()
		t, text = s.scanComment()

	case ch == '"' || ch == '\'' || ch == '`':
		s.unread()
		t, text, val = s.scanString(ch == '`')

	case ch == '.':
		next := s.peek()
		if isNumber(next) {
			t, text, val = s.scanNumber(ch)
			break
		} else if next == '.' {
			s.read()
			next = s.read()
			if next == '.' {
				t = TokenTypeEllipsis
				text = "..."
				break
			}
			s.unread()
			text = ".."
			break
		}
		t = TokenTypePERIOD
		text = string(ch)

	case ch == '\\':
		t = TokenTypeBACKSLASH
		text = string(ch)

	case ch == ',':
		t = TokenTypeCOMMA
		text = string(ch)

	case ch == ':':
		t = TokenTypeCOLON
		text = string(ch)

	case ch == '\n' || ch == '\r':
		t = TokenTypeWhitespace
		text = string(ch)

	case ch == ';':
		t = TokenTypeSEMICOLON
		text = string(ch)

	case ch == '+':
		t = TokenTypeADD
		text = string(ch)
		if s.read() == '+' {
			t = TokenTypeIncrement
			text = "++"
		} else {
			s.unread()
		}

	case ch == '-':
		t = TokenTypeSUB
		text = string(ch)
		if s.read() == '-' {
			t = TokenTypeDecrement
			text = "--"
		} else {
			s.unread()
		}

	case ch == '*':
		t = TokenTypeASTERISK
		text = string(ch)

	case ch == '&':
		t = TokenTypeAMPERSAND
		text = string(ch)

	case ch == '(':
		t = TokenTypeLPAREN
		text = string(ch)

	case ch == '[':
		t = TokenTypeLBRACK
		text = string(ch)

	case ch == '<':
		t = TokenTypeLess
		text = string(ch)
		if s.read() == '=' {
			t = TokenTypeLessOrEqual
			text = "<="
		} else {
			s.unread()
		}

	case ch == '{':
		t = TokenTypeLBRACE
		text = string(ch)

	case ch == ')':
		t = TokenTypeRPAREN
		text = string(ch)

	case ch == ']':
		t = TokenTypeRBRACK
		text = string(ch)

	case ch == '>':
		t = TokenTypeGreater
		text = string(ch)
		if s.read() == '=' {
			t = TokenTypeGreaterOrEqual
			text = ">="
		} else {
			s.unread()
		}

	case ch == '}':
		t = TokenTypeRBRACE
		text = string(ch)

	case ch == '#':
		t = TokenTypeHASHBANG
		text = string(ch)

	case ch == '?':
		t = TokenTypeQUESTIONMARK
		text = string(ch)

	case ch == '!':
		t = TokenTypeEXCL
		text = string(ch)
		if s.read() == '=' {
			t = TokenTypeNotEqual
			text = "!="
		} else {
			s.unread()
		}

	case ch == '=':
		t = TokenTypeASSIGN
		text = string(ch)
		next := s.read()
		if next == '=' {
			t = TokenTypeEqual
			text = "=="
		} else if next == '>' {
			t = TokenTypeArrow
			text = "=>"
		} else {
			s.unread()
		}

	case ch == eof:
		t = TokenTypeEOF

	default:
		// Unknown token
		text = string(ch)
	}

	token.Text = text
	token.Type = t
	token.Value = val
	token.EndColumn = s.pos.column
	token.EndLine = s.pos.line

	return
}

func (s *Scanner) scanComment() (t TokenType, text string) {
	var buf bytes.Buffer

	start := s.read()
	buf.WriteRune(start)

	afterStart := s.peek()
	if afterStart != '*' && afterStart != '/' {
		// Not a comment. Lets just return the slash
		t = TokenTypeSLASH
		text = string(buf.Bytes())
		return
	}

loop:
	for {
		ch := s.read()
		switch {
		case ch == eof:
			s.unread()
			if afterStart != '/' {
				return TokenTypeUnknown, buf.String()
			}

			// Single line comment ended
			break loop
		case afterStart == '/' && ch == '\n':
			// Single line comment ended
			s.unread()
			break loop
		case afterStart == '*' && ch == '*':
			buf.WriteRune(ch)
			next := s.read()

			if next == '/' {
				// Block comment ended
				buf.WriteRune(next)
				break loop
			} else {
				s.unread()
			}
		default:
			buf.WriteRune(ch)
		}
	}

	return TokenTypeComment, buf.String()
}

func (s *Scanner) scanString(rawString bool) (TokenType, string, string) {
	var buf bytes.Buffer
	var val bytes.Buffer

	start := s.read()
	buf.WriteRune(start)

	checkRune := func(value rune) {
		// TODO handle multiple runes
		val.WriteRune(value)
	}

loop:
	for {
		ch := s.read()
		switch {
		case !rawString && ch == '\n':
			s.unread()
			s.error("Line breaks not allowed on intepreted string literals")
			return TokenTypeUnknown, buf.String(), ""

		case ch == eof:
			s.unread()
			s.error("EOF before string closed")
			return TokenTypeUnknown, buf.String(), ""

		case !rawString && ch == '\\':
			buf.WriteRune(ch)
			next := s.read()
			if next == eof {
				s.unread()
				continue loop
			}

			switch next {
			case start:
				val.WriteRune(next)

			case '\\':
				val.WriteRune(next)

			case '0', '1', '2', '3', '4', '5', '6', '7': // Scan octal
				s.unread()
				t, v := s.scanDigits(8, 3)
				checkRune(v)
				buf.Write(t)
				continue loop

			case 'x':
				buf.WriteRune(next)
				t, v := s.scanDigits(16, 2)
				checkRune(v)
				buf.Write(t)
				continue loop

			case 'u':
				buf.WriteRune(next)
				t, v := s.scanDigits(16, 4)
				checkRune(v)
				buf.Write(t)
				continue loop

			case 'U':
				buf.WriteRune(next)
				t, v := s.scanDigits(16, 8)
				checkRune(v)
				buf.Write(t)
				continue loop

			default:
				val.WriteRune(ch)
				val.WriteRune(next)
			}

			buf.WriteRune(next)
		default:
			buf.WriteRune(ch)
			if ch == start {
				break loop
			} else {
				val.WriteRune(ch)
			}
		}
	}

	return TokenTypeString, buf.String(), val.String()
}

func (s *Scanner) scanDigits(base, n int) ([]byte, rune) {
	var buf bytes.Buffer
	result := 0
	var ch rune
	for n > 0 {
		ch = s.read()
		digVal := digitVal(ch)

		if digVal >= base {
			s.unread()
			break
		}
		buf.WriteRune(ch)
		result += digVal * int(math.Pow(float64(base), float64(n-1)))
		n--
	}

	if n > 0 {
		s.error("illegal char escape")
	}

	return buf.Bytes(), rune(result)
}

func (s *Scanner) scanIdent() (t TokenType, text string, val interface{}) {
	var buf bytes.Buffer
	t = TokenTypeIdent

	for {
		if ch := s.read(); ch == eof {
			s.unread()
			break
		} else if !isLetter(ch) && !isNumber(ch) && ch != '_' {
			s.unread()
			break
		} else {
			buf.WriteRune(ch)
		}
	}

	text = buf.String()
	switch text {
	case "true", "false":
		t = TokenTypeBoolean
		val = text == "true"
	}

	return
}

func (s *Scanner) scanNumber(ch rune) (t TokenType, text string, val interface{}) {
	var buf bytes.Buffer
	t = TokenTypeNumber

loop:
	for {
		switch {
		case isNumber(ch):
			buf.WriteRune(ch)
		case ch == '.' && t == TokenTypeNumber:
			t = TokenTypeFloat
			buf.WriteRune(ch)
		default:
			s.unread()
			break loop
		}

		ch = s.read()
	}

	text = buf.String()

	var err error
	if t == TokenTypeNumber {
		val, err = strconv.ParseInt(text, 10, 64)
		if err != nil {
			s.error(err.Error())
		}
	} else {
		val, err = strconv.ParseFloat(text, 64)
		if err != nil {
			s.error(err.Error())
		}
	}

	return t, text, val
}

func (s *Scanner) scanWhitespace() (t TokenType, text string) {
	var buf bytes.Buffer

	for {
		if ch := s.read(); ch == eof || !isWhitespace(ch) {
			s.unread()
			break
		} else {
			buf.WriteRune(ch)
		}
	}

	return TokenTypeWhitespace, buf.String()
}

func (s *Scanner) read() rune {
	s.lastPos = s.pos
	ch, _, err := s.r.ReadRune()
	if err != nil {
		return eof
	}
	if ch == '\n' {
		s.pos.column = 0
		s.pos.line++
	} else {
		s.pos.column++
	}
	return ch
}

func (s *Scanner) error(err string) {
	if s.Error != nil {
		s.Error(err)
	}
}

func (s *Scanner) peek() rune {
	defer s.unread()
	return s.read()
}

// unread places the previously read rune back on the reader.
func (s *Scanner) unread() {
	s.pos = s.lastPos
	_ = s.r.UnreadRune()
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isNumber(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t'
}

func digitVal(ch rune) int {
	switch {
	case '0' <= ch && ch <= '9':
		return int(ch - '0')
	case 'a' <= ch && ch <= 'f':
		return int(ch - 'a' + 10)
	case 'A' <= ch && ch <= 'F':
		return int(ch - 'A' + 10)
	}
	return 16
}

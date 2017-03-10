package scanner

import (
	"bufio"
	"bytes"
	"io"
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
	token.Column = s.pos.column
	token.Line = s.pos.line

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

	case isNumber(ch):
		t, text, val = s.scanNumber(ch)

	case ch == '/':
		s.unread()
		t, text = s.scanComment()

	case ch == '"' || ch == '\'':
		s.unread()
		t, text, val = s.scanString()

	case ch == '.':
		next := s.peek()
		if isNumber(next) {
			t, text, val = s.scanNumber(ch)
			break
		}
		t = TokenTypePERIOD
		text = string(ch)

	case ch == ',':
		t = TokenTypeCOMMA
		text = string(ch)

	case ch == ':':
		t = TokenTypeCOLON
		text = string(ch)

	case ch == ';':
		t = TokenTypeSEMICOLON
		text = string(ch)

	case ch == '+':
		t = TokenTypeADD
		text = string(ch)

	case ch == '-':
		t = TokenTypeSUB
		text = string(ch)

	case ch == '*':
		t = TokenTypeASTERIX
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
		t = TokenTypeLCHEV
		text = string(ch)

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
		t = TokenTypeRCHEV
		text = string(ch)

	case ch == '}':
		t = TokenTypeRBRACE
		text = string(ch)

	case ch == '#':
		t = TokenTypeHASHBANG
		text = string(ch)

	case ch == '!':
		t = TokenTypeEXCL
		text = string(ch)

	case ch == '=':
		t = TokenTypeASSIGN
		text = string(ch)

	case ch == eof:
		t = TokenTypeEOF
	}

	token.Text = text
	token.Type = t
	token.Value = val

	return
}

func (s *Scanner) scanComment() (t TokenType, text string) {
	var buf bytes.Buffer

	start := s.read()
	buf.WriteRune(start)

	afterStart := s.peek()

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

func (s *Scanner) scanString() (TokenType, string, string) {
	var buf bytes.Buffer
	var val bytes.Buffer

	start := s.read()
	buf.WriteRune(start)

loop:
	for {
		ch := s.read()
		switch ch {
		case eof:
			s.unread()
			return TokenTypeUnknown, buf.String(), ""
		case '\\':
			buf.WriteRune(ch)
			next := s.read()
			if next == eof {
				s.unread()
				continue loop
			}
			if next != start && next != '\\' {
				val.WriteRune(ch)
			}
			val.WriteRune(next)
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

func (s *Scanner) scanIdent() (t TokenType, text string, val interface{}) {
	var buf bytes.Buffer
	t = TokenTypeIdent

	for {
		if ch := s.read(); ch == eof {
			s.unread()
			break
		} else if !isLetter(ch) && !isNumber(ch) {
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

	if t == TokenTypeNumber {
		// TODO process error
		val, _ = strconv.ParseInt(text, 10, 64)
	} else {
		// TODO process error
		val, _ = strconv.ParseFloat(text, 64)
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
	return ch == ' ' || ch == '\t' || ch == '\n'
}

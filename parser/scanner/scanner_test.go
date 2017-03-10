package scanner

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
)

var tests = []struct {
	src     string
	results []Token
}{
	{
		src: "export fn\nfn1",
		results: []Token{
			Token{Type: TokenTypeIdent, Text: "export"},
			Token{Type: TokenTypeWhitespace, Column: 6, Text: " "},
			Token{Type: TokenTypeIdent, Column: 7, Text: "fn"},
			Token{Type: TokenTypeWhitespace, Column: 9, Text: "\n"},
			Token{Type: TokenTypeIdent, Line: 1, Text: "fn1"},
			Token{Type: TokenTypeEOF, Line: 1, Column: 3, Text: ""},
		},
	},
	{
		src: "\"foobar\"\n\"foo",
		results: []Token{
			Token{Type: TokenTypeString, Text: `"foobar"`, Value: "foobar"},
			Token{Type: TokenTypeWhitespace, Column: 8, Text: "\n"},
			Token{Type: TokenTypeUnknown, Line: 1, Column: 0, Text: `"foo`, Value: ""},
			Token{Type: TokenTypeEOF, Line: 1, Column: 4, Text: ``},
		},
	},
	{
		src: `"foo\"bar""foo\"bar"`,
		results: []Token{
			Token{Type: TokenTypeString, Text: `"foo\"bar"`, Value: `foo"bar`},
			Token{Type: TokenTypeString, Column: 10, Text: `"foo\"bar"`, Value: `foo"bar`},
			Token{Type: TokenTypeEOF, Line: 0, Column: 20, Text: ``},
		},
	},
	{
		src: "\"foo\\",
		results: []Token{
			Token{Type: TokenTypeUnknown, Text: `"foo\`, Value: ""},
			Token{Type: TokenTypeEOF, Line: 0, Column: 5, Text: ``},
		},
	},
	{
		src: "[]{},.:;+-=*&()<>!#",
		results: []Token{
			Token{Type: TokenTypeLBRACK, Column: 0, Text: `[`},
			Token{Type: TokenTypeRBRACK, Column: 1, Text: `]`},
			Token{Type: TokenTypeLBRACE, Column: 2, Text: `{`},
			Token{Type: TokenTypeRBRACE, Column: 3, Text: `}`},
			Token{Type: TokenTypeCOMMA, Column: 4, Text: `,`},
			Token{Type: TokenTypePERIOD, Column: 5, Text: `.`},
			Token{Type: TokenTypeCOLON, Column: 6, Text: `:`},
			Token{Type: TokenTypeSEMICOLON, Column: 7, Text: `;`},
			Token{Type: TokenTypeADD, Column: 8, Text: `+`},
			Token{Type: TokenTypeSUB, Column: 9, Text: `-`},
			Token{Type: TokenTypeASSIGN, Column: 10, Text: `=`},
			Token{Type: TokenTypeASTERIX, Column: 11, Text: `*`},
			Token{Type: TokenTypeAMPERSAND, Column: 12, Text: `&`},
			Token{Type: TokenTypeLPAREN, Column: 13, Text: `(`},
			Token{Type: TokenTypeRPAREN, Column: 14, Text: `)`},
			Token{Type: TokenTypeLCHEV, Column: 15, Text: `<`},
			Token{Type: TokenTypeRCHEV, Column: 16, Text: `>`},
			Token{Type: TokenTypeEXCL, Column: 17, Text: `!`},
			Token{Type: TokenTypeHASHBANG, Column: 18, Text: `#`},
			Token{Type: TokenTypeEOF, Column: 19, Text: ``},
		},
	},
	{
		src: "false true",
		results: []Token{
			Token{Type: TokenTypeBoolean, Column: 0, Text: `false`, Value: false},
			Token{Type: TokenTypeWhitespace, Column: 5, Text: ` `},
			Token{Type: TokenTypeBoolean, Column: 6, Text: `true`, Value: true},
			Token{Type: TokenTypeEOF, Column: 10, Text: ``},
		},
	},
	{
		src: "12348 1234.5 1234.5.5", // Sounds like a hack but I'll leave it to the parser to decide what to do
		results: []Token{
			Token{Type: TokenTypeNumber, Column: 0, Text: `12348`, Value: int64(12348)},
			Token{Type: TokenTypeWhitespace, Column: 5, Text: ` `},
			Token{Type: TokenTypeFloat, Column: 6, Text: `1234.5`, Value: 1234.5},
			Token{Type: TokenTypeWhitespace, Column: 12, Text: ` `},
			Token{Type: TokenTypeFloat, Column: 13, Text: `1234.5`, Value: 1234.5},
			Token{Type: TokenTypeFloat, Column: 19, Text: `.5`, Value: 0.5},
			Token{Type: TokenTypeEOF, Column: 21, Text: ``},
		},
	},
	{
		src: "// This is a comment\n/*\nfoo\n*/// eof comment",
		results: []Token{
			Token{Type: TokenTypeComment, Column: 0, Text: `// This is a comment`},
			Token{Type: TokenTypeWhitespace, Column: 20, Text: "\n"},
			Token{Type: TokenTypeComment, Column: 0, Line: 1, Text: "/*\nfoo\n*/"},
			Token{Type: TokenTypeComment, Column: 2, Line: 3, Text: `// eof comment`},
			Token{Type: TokenTypeEOF, Column: 16, Line: 3},
		},
	},
	{
		src: "/* eof ending block comments wont work",
		results: []Token{
			Token{Type: TokenTypeUnknown, Column: 0, Text: `/* eof ending block comments wont work`},
			Token{Type: TokenTypeEOF, Column: 38, Line: 0},
		},
	},
}

func TestScannerTable(t *testing.T) {
	for _, test := range tests {
		s := NewScanner(strings.NewReader(test.src))
		tokens := []Token{}
		for token := range s.ScanChannel() {
			tokens = append(tokens, token)
			if token.Type == TokenTypeEOF {
				break
			}
		}

		once := sync.Once{}

		for i, token := range tokens {
			var expectedToken Token
			if len(test.results) > i {
				expectedToken = test.results[i]
			} else {
				t.Error("Too many token returned")
			}
			if !reflect.DeepEqual(expectedToken, token) {
				once.Do(func() {
					t.Error("source", test.src)
				})
				t.Errorf("Token (%d): %#v didn't match expected %#v", i, token, expectedToken)
			}
		}

	}
}

func TestTokenizesC(t *testing.T) {
	s := NewScanner(strings.NewReader(`
    #include <stdio.h>
    int main()
    {
      // printf() displays the string inside quotation
      printf("Hello, World!");
      return 0;
    }
  `))

	for token := range s.ScanChannel() {
		if token.Type == TokenTypeUnknown {
			t.Errorf("Encountered an unknown token %s", token)
		}
	}
}

func TestTokenizesGO(t *testing.T) {
	s := NewScanner(strings.NewReader(`
    package main
    import "fmt"
    func main() {
      fmt.Println("hello world")
    }
  `))

	for token := range s.ScanChannel() {
		if token.Type == TokenTypeUnknown {
			t.Errorf("Encountered an unknown token %s", token)
		}
	}
}

func ExampleNewScanner() {
	s := NewScanner(strings.NewReader(`
    fn test(foo: Int, bar: Test) Foo { // 1
      var a = foo; // 2
      var b = 123; // 3
      var c = true; // 4
      // This is a comment 5
      var d = 123.5; // 6
      /* One line block comment */ // 7
      /* 8
       * Multiline comment 9
       */ // 10
      return "foo"; // 11
    } // 12
  `))
	tokens := []string{}
	for token := range s.ScanChannel() {
		if token.Type != TokenTypeWhitespace {
			tokens = append(tokens, strings.Replace(token.String(), "\n", "[NL]", -1))
		}
	}

	fmt.Print(strings.Join(tokens, "\n"))
	// Output:
	// 1:4 IDENT(fn)
	// 1:7 IDENT(test)
	// 1:11 LPAREN(()
	// 1:12 IDENT(foo)
	// 1:15 COLON(:)
	// 1:17 IDENT(Int)
	// 1:20 COMMA(,)
	// 1:22 IDENT(bar)
	// 1:25 COLON(:)
	// 1:27 IDENT(Test)
	// 1:31 RPAREN())
	// 1:33 IDENT(Foo)
	// 1:37 LBRACE({)
	// 1:39 COMMENT(// 1)
	// 2:6 IDENT(var)
	// 2:10 IDENT(a)
	// 2:12 ASSIGN(=)
	// 2:14 IDENT(foo)
	// 2:17 SEMICOLON(;)
	// 2:19 COMMENT(// 2)
	// 3:6 IDENT(var)
	// 3:10 IDENT(b)
	// 3:12 ASSIGN(=)
	// 3:14 NUMBER(123)
	// 3:17 SEMICOLON(;)
	// 3:19 COMMENT(// 3)
	// 4:6 IDENT(var)
	// 4:10 IDENT(c)
	// 4:12 ASSIGN(=)
	// 4:14 BOOL(true)
	// 4:18 SEMICOLON(;)
	// 4:20 COMMENT(// 4)
	// 5:6 COMMENT(// This is a comment 5)
	// 6:6 IDENT(var)
	// 6:10 IDENT(d)
	// 6:12 ASSIGN(=)
	// 6:14 FLOAT(123.5)
	// 6:19 SEMICOLON(;)
	// 6:21 COMMENT(// 6)
	// 7:6 COMMENT(/* One line block comment */)
	// 7:35 COMMENT(// 7)
	// 8:6 COMMENT(/* 8[NL]       * Multiline comment 9[NL]       */)
	// 10:10 COMMENT(// 10)
	// 11:6 IDENT(return)
	// 11:13 STRING(foo)
	// 11:18 SEMICOLON(;)
	// 11:20 COMMENT(// 11)
	// 12:4 RBRACE(})
	// 12:6 COMMENT(// 12)
	// 13:2 EOF()
}

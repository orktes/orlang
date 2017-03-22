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
			Token{Type: TokenTypeWhitespace, StartColumn: 6, Text: " "},
			Token{Type: TokenTypeIdent, StartColumn: 7, Text: "fn"},
			Token{Type: TokenTypeWhitespace, StartColumn: 9, Text: "\n"},
			Token{Type: TokenTypeIdent, StartLine: 1, Text: "fn1"},
			Token{Type: TokenTypeEOF, StartLine: 1, StartColumn: 3, Text: ""},
		},
	},
	{
		src: "\"foobar\"\n\"foo",
		results: []Token{
			Token{Type: TokenTypeString, Text: `"foobar"`, Value: "foobar"},
			Token{Type: TokenTypeWhitespace, StartColumn: 8, Text: "\n"},
			Token{Type: TokenTypeUnknown, StartLine: 1, StartColumn: 0, Text: `"foo`, Value: ""},
			Token{Type: TokenTypeEOF, StartLine: 1, StartColumn: 4, Text: ``},
		},
	},
	{
		src: `"foo\"bar""foo\"bar"`,
		results: []Token{
			Token{Type: TokenTypeString, Text: `"foo\"bar"`, Value: `foo"bar`},
			Token{Type: TokenTypeString, StartColumn: 10, Text: `"foo\"bar"`, Value: `foo"bar`},
			Token{Type: TokenTypeEOF, StartLine: 0, StartColumn: 20, Text: ``},
		},
	},
	{
		src: "\"foo\\",
		results: []Token{
			Token{Type: TokenTypeUnknown, Text: `"foo\`, Value: ""},
			Token{Type: TokenTypeEOF, StartLine: 0, StartColumn: 5, Text: ``},
		},
	},
	{
		src: `"foo\nbar"`,
		results: []Token{
			Token{Type: TokenTypeString, Text: `"foo\nbar"`, Value: "foo\\nbar"},
			Token{Type: TokenTypeEOF, StartLine: 0, StartColumn: 10, Text: ``},
		},
	},
	{
		src: `"foo\\bar"`,
		results: []Token{
			Token{Type: TokenTypeString, Text: `"foo\\bar"`, Value: `foo\bar`},
			Token{Type: TokenTypeEOF, StartLine: 0, StartColumn: 10, Text: ``},
		},
	},
	{
		src: "\"\\123\\x53\\u2318\"",
		results: []Token{
			Token{Type: TokenTypeString, Text: "\"\\123\\x53\\u2318\"", Value: "SS⌘"},
			Token{Type: TokenTypeEOF, StartLine: 0, StartColumn: 16, Text: ``},
		},
	},
	/*
		{
			src: `"\xe6\x97\xa5\xe6\x9c\xac\xe8\xaa\x9e"`,
			results: []Token{
				Token{Type: TokenTypeString, Text: `"\xe6\x97\xa5\xe6\x9c\xac\xe8\xaa\x9e"`, Value: "日本語"},
				Token{Type: TokenTypeEOF, StartLine: 0, StartColumn: 38, Text: ``},
			},
		},
	*/
	{
		src: "`\\n\\000`",
		results: []Token{
			Token{Type: TokenTypeString, Text: "`\\n\\000`", Value: "\\n\\000"},
			Token{Type: TokenTypeEOF, StartLine: 0, StartColumn: 8, Text: ``},
		},
	},
	{
		src: "\"\n\"",
		results: []Token{
			Token{Type: TokenTypeUnknown, Text: "\"", Value: ""},
			Token{Type: TokenTypeWhitespace, StartLine: 0, StartColumn: 1, Text: "\n", Value: nil},
			Token{Type: TokenTypeUnknown, StartLine: 1, StartColumn: 0, Text: "\"", Value: ""},
			Token{Type: TokenTypeEOF, StartLine: 1, StartColumn: 1, Text: ``},
		},
	},
	{
		src: "[]{},.:;+-=*&()<>!#",
		results: []Token{
			Token{Type: TokenTypeLBRACK, StartColumn: 0, Text: `[`},
			Token{Type: TokenTypeRBRACK, StartColumn: 1, Text: `]`},
			Token{Type: TokenTypeLBRACE, StartColumn: 2, Text: `{`},
			Token{Type: TokenTypeRBRACE, StartColumn: 3, Text: `}`},
			Token{Type: TokenTypeCOMMA, StartColumn: 4, Text: `,`},
			Token{Type: TokenTypePERIOD, StartColumn: 5, Text: `.`},
			Token{Type: TokenTypeCOLON, StartColumn: 6, Text: `:`},
			Token{Type: TokenTypeSEMICOLON, StartColumn: 7, Text: `;`},
			Token{Type: TokenTypeADD, StartColumn: 8, Text: `+`},
			Token{Type: TokenTypeSUB, StartColumn: 9, Text: `-`},
			Token{Type: TokenTypeASSIGN, StartColumn: 10, Text: `=`},
			Token{Type: TokenTypeASTERIX, StartColumn: 11, Text: `*`},
			Token{Type: TokenTypeAMPERSAND, StartColumn: 12, Text: `&`},
			Token{Type: TokenTypeLPAREN, StartColumn: 13, Text: `(`},
			Token{Type: TokenTypeRPAREN, StartColumn: 14, Text: `)`},
			Token{Type: TokenTypeLess, StartColumn: 15, Text: `<`},
			Token{Type: TokenTypeGreater, StartColumn: 16, Text: `>`},
			Token{Type: TokenTypeEXCL, StartColumn: 17, Text: `!`},
			Token{Type: TokenTypeHASHBANG, StartColumn: 18, Text: `#`},
			Token{Type: TokenTypeEOF, StartColumn: 19, Text: ``},
		},
	},
	{
		src: "false true",
		results: []Token{
			Token{Type: TokenTypeBoolean, StartColumn: 0, Text: `false`, Value: false},
			Token{Type: TokenTypeWhitespace, StartColumn: 5, Text: ` `},
			Token{Type: TokenTypeBoolean, StartColumn: 6, Text: `true`, Value: true},
			Token{Type: TokenTypeEOF, StartColumn: 10, Text: ``},
		},
	},
	{
		src: "12348 1234.5 1234.5.5", // Sounds like a hack but I'll leave it to the parser to decide what to do
		results: []Token{
			Token{Type: TokenTypeNumber, StartColumn: 0, Text: `12348`, Value: int64(12348)},
			Token{Type: TokenTypeWhitespace, StartColumn: 5, Text: ` `},
			Token{Type: TokenTypeFloat, StartColumn: 6, Text: `1234.5`, Value: 1234.5},
			Token{Type: TokenTypeWhitespace, StartColumn: 12, Text: ` `},
			Token{Type: TokenTypeFloat, StartColumn: 13, Text: `1234.5`, Value: 1234.5},
			Token{Type: TokenTypeFloat, StartColumn: 19, Text: `.5`, Value: 0.5},
			Token{Type: TokenTypeEOF, StartColumn: 21, Text: ``},
		},
	},
	{
		src: "// This is a comment\n/*\nfoo\n*/// eof comment",
		results: []Token{
			Token{Type: TokenTypeComment, StartColumn: 0, Text: `// This is a comment`},
			Token{Type: TokenTypeWhitespace, StartColumn: 20, Text: "\n"},
			Token{Type: TokenTypeComment, StartColumn: 0, StartLine: 1, Text: "/*\nfoo\n*/"},
			Token{Type: TokenTypeComment, StartColumn: 2, StartLine: 3, Text: `// eof comment`},
			Token{Type: TokenTypeEOF, StartColumn: 16, StartLine: 3},
		},
	},
	{
		src: "/* eof ending block comments wont work",
		results: []Token{
			Token{Type: TokenTypeUnknown, StartColumn: 0, Text: `/* eof ending block comments wont work`},
			Token{Type: TokenTypeEOF, StartColumn: 38, StartLine: 0},
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

			if len(test.results) > i+1 {
				nextToken := test.results[i+1]
				expectedToken.EndColumn = nextToken.StartColumn
				expectedToken.EndLine = nextToken.StartLine
			} else {
				expectedToken.EndColumn = expectedToken.StartColumn
				expectedToken.EndLine = expectedToken.StartLine
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

func BenchmarkScannerTable(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, test := range tests {
			s := NewScanner(strings.NewReader(test.src))
			for range s.ScanChannel() {
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

func BenchmarkTokenizesC(b *testing.B) {
	for i := 0; i < b.N; i++ {
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
				b.Errorf("Encountered an unknown token %s", token)
			}
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

func TestTokenizesJSX(t *testing.T) {
	s := NewScanner(strings.NewReader(`
    <div>
        <h3>TODO</h3>
        <TodoList items={this.state.items} />
        <form onSubmit={this.handleSubmit}>
          <input onChange={this.handleChange} value={this.state.text} />
          <button>{'Add #' + (this.state.items.length + 1)}</button>
        </form>
      </div>
  `))

	for token := range s.ScanChannel() {
		if token.Type == TokenTypeUnknown {
			t.Errorf("Encountered an unknown token %s", token)
		}
	}
}

func TestTokenizesJSON(t *testing.T) {
	s := NewScanner(strings.NewReader(`
    {
    "glossary": {
        "title": "example glossary",
		"GlossDiv": {
            "title": "S",
			"GlossList": {
                "GlossEntry": {
                    "ID": "SGML",
					"SortAs": "SGML",
					"GlossTerm": "Standard Generalized Markup Language",
					"Acronym": "SGML",
					"Abbrev": "ISO 8879:1986",
					"GlossDef": {
                        "para": "A meta-markup language, used to create markup languages such as DocBook.",
						"GlossSeeAlso": ["GML", "XML"]
                    },
					"GlossSee": "markup"
                }
            }
        }
    }
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

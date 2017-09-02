package parser

import (
	"strings"
	"testing"
)

func TestParseFailures(t *testing.T) {
	tests := []struct {
		src string
		err string
	}{
		// Undefined token
		{"unexpected", "1:1: Unexpected token IDENT(unexpected)"},
		{"bar () {}", "1:1: Unexpected token IDENT(bar)"},
		// Invalid functions
		{"fn test{}", "1:8: Expected [LPAREN] got LBRACE"},
		{"fn test()", "1:10: Expected code block got EOF"},
		{"fn test(,) {}", "1:9: Expected [IDENT RPAREN] got COMMA"},
		{"fn test(bar i = 1) {}", "1:13: Expected [RPAREN COMMA] got i"},
		{"fn test(int) : {}", "1:16: Expected function return type got LBRACE({)"},
		{"fn test(foo : float foo) {}", "1:21: Expected [RPAREN COMMA] got foo"},
		{"fn test(foo : ) {}", "1:15: Expected [IDENT] got RPAREN"},
		{"fn test(foo : bar = ) {}", "1:21: Expected expression got RPAREN())"},
		{"fn test(foo : int) {]", "1:21: Expected code block got RBRACK(])"},
		{"fn", "1:3: Expected function name or argument list got EOF"},
		{"fn (foo : int) {}", "1:17: Root level functions can't be anonymous"},
		{"fn ( {}", "1:6: Expected [IDENT RPAREN] got LBRACE"},
		// Variable declarations
		{"var [", "1:5: Expected variable or tuple declaration got LBRACK([)"},
		{"var foo = (1", "1:13: Expected [RPAREN] got EOF"},
		{"var foo = ()", "1:12: Expected expression got RPAREN())"},
		{"var foo = (1,)", "1:14: Expected expression got RPAREN())"},
		{"var foo : (int, int", "1:20: Expected [RPAREN] got EOF"},
		{"var foo : ()", "1:12: Expected type got RPAREN())"},
		{"var foo : (int,)", "1:16: Expected type got RPAREN())"},
		{"var (foo) :", "1:12: Expected type got EOF"},
		{"var foo :", "1:10: Expected type got EOF"},
		{"var (", "1:6: Expected [RPAREN] got EOF"},
		{"var foo : (int32, float32) :", "1:29: Expected function return type got EOF"},
		{"var foo : [int32", "1:17: Expected [RBRACK] got EOF"},
		{"var foo : [", "1:12: Expected length expression got EOF"},
		// For loops
		{"fn foobar() { for var i = 0; i; [] }", "1:36: Expected array type got RBRACE(})"},
		{"fn foobar() { for var i = 0; {}}", "1:30: Expected expression got LBRACE({)"},
		{"fn foobar() { for var i = 0; true {}}", "1:35: Expected ; got LBRACE({)"},
		{"fn foobar() { for }", "1:19: Expected statement, ; or code block got RBRACE(})"},
		{"fn foobar() { for true true {} }", "1:24: Expected ; or code block got BOOL(true)"},
		{"fn foobar() { foo = , }", "1:21: Expected expression got COMMA(,)"},
		// Arrays
		{"var foo : []", "1:13: Expected array type got EOF"},
		{"var foo : []int32 = []", "1:23: Expected array type got EOF"},
		{"var foo : []int32 = []int32", "1:28: Expected [LBRACE] got EOF"},
		{"var foo : []int32 = []int32{", "1:29: Expected expression list or left brace got EOF"},
		{"var foo : []int32 = []int32{1", "1:30: Expected [RBRACE] got EOF"},
		// If statemts
		{"fn foobar() {  if }", "1:19: Expected expression got RBRACE(})"},
		{"fn foobar() {  if 1 < {} }", "1:23: Expected expression got LBRACE({)"},
		{"fn foobar() {  if 1 ! {} }", "1:21: Expected code block got EXCLAMATION(!)"},
		{"fn foobar() {  if true foo }", "1:24: Expected code block got IDENT(foo)"},
		{"fn foobar() {  if true {} else f", "1:32: Expected if statement or code block got IDENT(f)"},
		// Function calls
		{"fn foobar() {  foobar(.) }", "1:23: Expected [RPAREN] got PERIOD"},
		{"fn foobar() {  fn foobar(i:int;) {} }", "1:31: Expected [RPAREN COMMA] got SEMICOLON"},
		// Member expressions
		{"fn foobar() {  foobar.false }", "1:23: Expected property name got BOOL(false)"},
		// Reservedkeyword
		{"fn return() {  }", "1:4: return is a reserved keyword"},
		{"fn foobar() { var fn = 1 }", "1:19: fn is a reserved keyword"},
		{"fn foobar() { foo.return }", "1:19: return is a reserved keyword"},
		{"fn foobar(fn: int) {  }", "1:11: fn is a reserved keyword"},
		{"fn foobar(int: fn) {  }", "1:16: fn is a reserved keyword"},
		{"fn foobar() { foobar(int:return) }", "1:26: return is a reserved keyword"},
		{"fn foobar() { foobar(return:0) }", "1:28: return is a reserved keyword"},
		// BinaryExpression
		{"fn foobar() { var foo = 1 + }", "1:29: Expected expression got RBRACE(})"},
		// UnaryExpression
		{"fn foobar() { var foo = - }", "1:27: Expected expression got RBRACE(})"},
		// Ellipsis
		{"fn foobar() { var foo = ... }", "1:25: Expected expression got ..."},
		// MacroSubstitutions inside normal code
		{"fn foobar() {var foo = $f}", "1:24: Could not find matching node for $f"},
		{"fn foobar() {$f}", "1:14: Could not find matching node for $f"},
		// Macros
		{"macro M { ($(foo),,*) : () }", "1:19: Macro repetition can only have one token as a delimiter"},
		{"macro M { () : () } fn main() { M!(foo) }", "1:36: No rules expected token IDENT(foo)"},
		{"macro M { () : () } fn main() { M!( }", "1:37: No rules expected token RBRACE(})"},
		{"macro M { () : ($()) } fn main() { M!() }", "1:20: Expected [ASTERISK] got RPAREN"},
		{"macro M { () : ($(", "1:19: Expected token but got eof"},
		{"macro M { (()) : ($(", "1:21: Expected token but got eof"},
		{"macro M { ($foo) : ()", "1:16: Expected [COLON] got RPAREN"},
		{"macro M { ($foo:) : ()", "1:17: Expected pattern key type got RPAREN())"},
		{"macro M { ($) : ()", "1:13: Expected macro pattern got RPAREN())"},
		{"macro { () : () }", "1:7: Expected macro name got LBRACE({)"},
		{"macro M () : () }", "1:9: Expected [LBRACE] got LPAREN"},
		{"macro M { () : }", "1:17: Expected [LPAREN LBRACE LBRACK] got EOF"},
		{"macro M { () : ($a) } fn main() { M!() }", "1:38: Could not find macro argument for metavariable $a"},
		{"macro M { () : ($a) } fn main() { M! }", "1:38: Could not find macro argument for metavariable $a"},
		{"macro M { ($(foo)+) : () } fn main() { M!(bar) }", "1:43: No rules expected token IDENT(bar)"},
		{"fn main() { M!(foo) }", "1:13: No macro with name M"},
		{"macro M { (", "1:12: Expected token but got eof"},
		{"macro M { ($()", "1:15: Expected macro repetition delimeter or operand (+, * or ?) got EOF"},
		// fuzz test results
		//{"var r=foo(0(//", ""},
	}
	for _, test := range tests {
		_, err := Parse(strings.NewReader(test.src))

		if err == nil {
			t.Errorf("Expected %s to return error %s, but got nothing", test.src, test.err)
		}

		if err.Error() != test.err {
			t.Errorf("Expected %s to return error %s, but got %s", test.src, test.err, err.Error())
		}
	}
}

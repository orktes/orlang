package parser

import (
	"strings"
	"testing"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/scanner"
)

func testScanner(src string) *scanner.Scanner {
	return scanner.NewScanner(strings.NewReader(src))
}

func TestParserRead(t *testing.T) {
	p := NewParser(testScanner("foobar"))

	if p.read().Text != "foobar" {
		t.Error("Wrong token returned")
	}

	if p.read().Type != scanner.TokenTypeEOF {
		t.Error("EOF should have been returned")
	}
}

func TestParserOnError(t *testing.T) {
	p := NewParser(nil)
	p.lastTokens = []scanner.Token{scanner.Token{}}

	p.Error = func(indx int, pos ast.Position, endPos ast.Position, msg string) {
		if indx != 0 {
			t.Error("Wrong index")
		}

		if pos.Column+pos.Line != 0 {
			t.Error("Wrong pos")
		}

		if msg != "Foobar" {
			t.Error("Wrong error")
		}
	}
	p.error("Foobar")
}

func TestParserUnreadRead(t *testing.T) {
	p := NewParser(testScanner("foobar"))

	if p.read().Text != "foobar" {
		t.Error("Wrong token returned")
	}

	p.unread()

	if p.read().Text != "foobar" {
		t.Error("Wrong token returned")
	}
}

func TestParserPeek(t *testing.T) {
	p := NewParser(testScanner("foobar"))

	if p.peek().Text != "foobar" {
		t.Error("Wrong token returned")
	}

	if p.peek().Text != "foobar" {
		t.Error("Wrong token returned")
	}

	if p.read().Text != "foobar" {
		t.Error("Wrong token returned")
	}
}

func TestParserPeekMultiple(t *testing.T) {
	p := NewParser(testScanner("foobar;barfoo;"))

	tokens := p.peekMultiple(3)

	if tokens[0].Text != "foobar" {
		t.Error("Wrong token returned")
	}

	if tokens[1].Text != ";" {
		t.Error("Wrong token returned", tokens[1].String())
	}

	if tokens[2].Text != "barfoo" {
		t.Error("Wrong token returned", tokens[2].String())
	}

	if p.read().Text != "foobar" {
		t.Error("Wrong token returned")
	}

	if p.read().Text != ";" {
		t.Error("Wrong token returned")
	}
}

func TestParserSkip(t *testing.T) {
	p := NewParser(testScanner("foobar;barfoo;"))
	p.skip()
	if p.read().Text != ";" {
		t.Error("Wrong token returned")
	}
}

func TestExpectPattern(t *testing.T) {
	p := NewParser(testScanner("foobar;barfoo;"))
	tokens, ok := p.expectPattern(
		scanner.TokenTypeIdent,
		scanner.TokenTypeSEMICOLON,
		scanner.TokenTypeIdent,
		scanner.TokenTypeSEMICOLON)

	if !ok {
		t.Error("Didnt get expected pattern")
	}

	if tokens[0].Type != scanner.TokenTypeIdent {
		t.Error("Didnt get expected pattern")
	}

	if tokens[1].Type != scanner.TokenTypeSEMICOLON {
		t.Error("Didnt get expected pattern")
	}

	if tokens[2].Type != scanner.TokenTypeIdent {
		t.Error("Didnt get expected pattern")
	}

	if tokens[3].Type != scanner.TokenTypeSEMICOLON {
		t.Error("Didnt get expected pattern")
	}

	_, ok = p.expectPattern(scanner.TokenTypeIdent)
	if ok {
		t.Error("Nothing should be returned")
	}

}

func TestReturnToBuffer(t *testing.T) {
	p := NewParser(testScanner("foobar;barfoo;"))
	p.read()

	tokens, _ := p.expectPattern(
		scanner.TokenTypeSEMICOLON,
		scanner.TokenTypeIdent,
		scanner.TokenTypeSEMICOLON)

	p.returnToBuffer(tokens)

	tokens, ok := p.expectPattern(
		scanner.TokenTypeSEMICOLON,
		scanner.TokenTypeIdent,
		scanner.TokenTypeSEMICOLON)

	if !ok {
		t.Error("Didnt get expected pattern")
	}

	if tokens[0].Type != scanner.TokenTypeSEMICOLON {
		t.Error("Didnt get expected pattern")
	}

	if tokens[1].Type != scanner.TokenTypeIdent {
		t.Error("Didnt get expected pattern")
	}

	if tokens[2].Type != scanner.TokenTypeSEMICOLON {
		t.Error("Didnt get expected pattern")
	}

}

func TestSnapshots(t *testing.T) {
	p := NewParser(testScanner("foobar;barfoo;"))
	p.snapshot()
	p.read()
	p.unread()
	p.read()
	p.snapshot()
	p.read()
	p.snapshot()
	p.read()
	p.snapshot()
	p.read()

	p.restore()
	p.restore()
	p.restore()
	p.restore()

	tokens, ok := p.expectPattern(
		scanner.TokenTypeIdent,
		scanner.TokenTypeSEMICOLON,
		scanner.TokenTypeIdent,
		scanner.TokenTypeSEMICOLON)

	if !ok {
		t.Error("Didnt get expected pattern")
	}

	if tokens[0].Text != "foobar" {
		t.Error("Didnt get expected pattern")
	}

	if tokens[1].Type != scanner.TokenTypeSEMICOLON {
		t.Error("Didnt get expected pattern")
	}

	if tokens[2].Type != scanner.TokenTypeIdent {
		t.Error("Didnt get expected pattern")
	}

	if tokens[3].Type != scanner.TokenTypeSEMICOLON {
		t.Error("Didnt get expected pattern")
	}

}

func TestExternDefinition(t *testing.T) {
	_, err := Parse(strings.NewReader(`
		extern printf(format: string, args:...) : int
	`))
	if err != nil {
		t.Error(err)
	}
}

func BenchmarkParserSimple(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Parse(strings.NewReader(`
			fn foobar() {}
		`))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParser(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, err := Parse(strings.NewReader(`
			fn foobar(x : int = 0, y: int = 0) {
				foobar()
				foobar()()
				foobar()()()
				someObj.foo()
				foobar(10, 20)
				foobar(x: 10, y: 20)
				fn barfoo() {
					var i : int = 10
					i = 20
					for barfoo(i: i) {
						fn barfoo() {
							var i : int = 10
							i = 20
							for barfoo(i: i) {
								fn barfoo() {
									var i : int = 10
									i = 20
									for barfoo(i: i) {
										fn barfoo() {
											var i : int = 10
											i = 20
											for barfoo(i: i) {
												fn barfoo() {
													var i : int = 10
													i = 20
													for barfoo(i: i) {
														fn barfoo() {
															var i : int = 10
															i = 20
															for barfoo(i: i) {
																fn barfoo() {
																	var i : int = 10
																	i = 20
																	for barfoo(i: i) {
																		fn barfoo() {
																			var i : int = 10
																			i = 20
																			for barfoo(i: i) {
																				fn barfoo() {
																					var i : int = 10
																					i = 20
																					for barfoo(i: i) {

																					}
																				}
																			}
																		}
																	}
																}
															}
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		`))
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkWalker(b *testing.B) {
	file, err := Parse(strings.NewReader(`
		fn foobar(x : int = 0, y: int = 0) {
			foobar()
			foobar()()
			foobar()()()
			someObj.foo()
			foobar(10, 20)
			foobar(x: 10, y: 20)
			fn barfoo() {
				var i : int = 10
				i = 20
				for barfoo(i: i) {
					fn barfoo() {
						var i : int = 10
						i = 20
						for barfoo(i: i) {
							fn barfoo() {
								var i : int = 10
								i = 20
								for barfoo(i: i) {
									fn barfoo() {
										var i : int = 10
										i = 20
										for barfoo(i: i) {
											fn barfoo() {
												var i : int = 10
												i = 20
												for barfoo(i: i) {
													fn barfoo() {
														var i : int = 10
														i = 20
														for barfoo(i: i) {
															fn barfoo() {
																var i : int = 10
																i = 20
																for barfoo(i: i) {
																	fn barfoo() {
																		var i : int = 10
																		i = 20
																		for barfoo(i: i) {
																			fn barfoo() {
																				var i : int = 10
																				i = 20
																				for barfoo(i: i) {

																				}
																			}
																		}
																	}
																}
															}
														}
													}
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	`))
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		count := 0
		var visitor ast.Visitor
		visitor = ast.VisitorFunc(func(ast.Node) ast.Visitor {
			count++
			return visitor
		})
		ast.Walk(visitor, file)

		if count != 226 {
			b.Error("Invalid node count")
		}
	}
}

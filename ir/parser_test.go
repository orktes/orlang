package ir

import (
	"strings"
	"testing"

	"github.com/orktes/orlang/scanner"
)

func TestParser(t *testing.T) {
	err := Parse(NewScanner(scanner.NewScanner(strings.NewReader(
		`
type Foo {int32, int32}

fn foobar(%x : int32, %y : int32) : ptr<Foo> {
  %temp0 = 10 : int32
  %temp1 = %x > %temp0 : bool

  br_cond %temp1, label0, label1

label0:
  %temp2 = 10 : int32
  %temp3 = alloc Foo : ptr<Foo>

  store %temp3, %temp2
  store %temp3, %y, 1

  return %temp3 : ptr<Foo>

label1:
  %temp4 = alloc Foo : ptr<Foo>

  store %temp3, %x
  store %temp3, %y, 1

  return %temp4 : ptr<Foo>
}
  `))))
	if err != nil {
		t.Error(err)
	}
}

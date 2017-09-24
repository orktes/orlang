package ir

import (
	"strings"
	"testing"

	"github.com/orktes/orlang/scanner"
)

func TestScannerExtensions(t *testing.T) {
	s := NewScanner(scanner.NewScanner(strings.NewReader("%temp1\n%temp2\n%bool\n%1")))
	if s.Scan().String() != "0:0 IDENT(%temp1)" {
		t.Error("Wrong token returned")
	}
	s.Scan()
	if s.Scan().String() != "1:0 IDENT(%temp2)" {
		t.Error("Wrong token returned")
	}

	s.Scan()
	if s.Scan().String() != "2:0 IDENT(%bool)" {
		t.Error("Wrong token returned")
	}

	s.Scan()
	if s.Scan().String() != "3:0 UNKNOWN(%)" {
		t.Error("Wrong token returned")
	}

	if s.Scan().String() != "3:1 NUMBER(1)" {
		t.Error("Wrong token returned")
	}
}

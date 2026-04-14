package llvm

import (
	"strings"
	"testing"

	"github.com/orktes/orlang/analyser"
	"github.com/orktes/orlang/parser"
)

func TestLLVMCodeGen(t *testing.T) {
	code := `
	fn main() {
		return 42
	}
	`
	file, err := parser.Parse(strings.NewReader(code))
	if err != nil {
		t.Fatal(err)
	}

	an, err := analyser.New(file)
	if err != nil {
		t.Fatal(err)
	}

	info, err := an.Analyse()
	if err != nil {
		t.Fatal(err)
	}

	codegen := New(info)
	ir := codegen.Generate(file)

	if !strings.Contains(ir, "define i32 @main()") {
		t.Errorf("Expected main function definition, got: %s", ir)
	}

	if !strings.Contains(ir, "ret i32 42") {
		t.Errorf("Expected return 42, got: %s", ir)
	}
}

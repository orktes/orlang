package e2e

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/orktes/orlang/analyser"
	"github.com/orktes/orlang/codegen/llvm"
	"github.com/orktes/orlang/parser"
)

func TestLLVME2E(t *testing.T) {
	// 1. Create a temporary directory for artifacts
	tmpDir, err := ioutil.TempDir("", "orlang_llvm_e2e")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Define simple Orlang code
	code := `
	extern puts(s: string)

	interface Resetable {
		fn reset()
	}

	struct Point {
		var x = 0
		var y = 0
		
		fn sum() => int32 {
			return this.x + this.y
		}

		fn reset() {
			this.x = 0
			this.y = 0
		}
	}

	fn reset(resetable : Resetable) {
		resetable.reset()
	}

	fn main() {
		var p = Point{10, 20}
		var s = p.sum()
		
		if s == 30 {
			puts("Sum is 30")
		}

		p.x = 100
		p.y = 200
		s = p.sum()
		if s != 300 {
			puts("Sum is not 300")
		}
		
		reset(p)

		s = p.sum()
		if s != 0 {
			puts("Point was not reset")
		}

		return s + 12
	}
	`

	// 3. Parse and Analyze
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

	// 4. Generate LLVM IR
	codegen := llvm.New(info)
	ir := codegen.Generate(file)

	println(ir)

	// 5. Write IR to file
	llPath := filepath.Join(tmpDir, "test.ll")
	if err := ioutil.WriteFile(llPath, []byte(ir), 0644); err != nil {
		t.Fatal(err)
	}

	// 6. Compile with clang
	exePath := filepath.Join(tmpDir, "test_exe")
	cmd := exec.Command("clang", "-Wno-override-module", "-o", exePath, llPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clang failed: %s\n%s", err, out)
	}

	// 7. Run executable
	cmd = exec.Command(exePath)
	out, err := cmd.CombinedOutput()

	// 8. Check output
	if !strings.Contains(string(out), "Sum is 30") {
		t.Errorf("Expected output to contain 'Sum is 30', got: %s", out)
	}

	// 9. Check exit code
	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ExitCode() != 42 {
			t.Errorf("Expected exit code 42, got %d", exitError.ExitCode())
		}
	} else if err != nil {
		t.Fatalf("Execution failed: %v", err)
	} else {
		t.Errorf("Expected exit code 42, got 0 (success)")
	}
}

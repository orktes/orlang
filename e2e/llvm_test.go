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
	"github.com/orktes/orlang/runtimelib"
)

func TestLLVMCodegenSmokeTest(t *testing.T) {
	// This is a basic smoke test to verify LLVM codegen works
	// The comprehensive tests are in e2e/1_simple, e2e/2_interfaces, etc.
	// and can be run with ./run.sh

	// 1. Create a temporary directory for artifacts
	tmpDir, err := ioutil.TempDir("", "orlang_llvm_smoke_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Define simple Orlang code
	code := `
	fn add(a: int32, b: int32) => int32 {
		return a + b
	}
	
	fn main() => int32 {
		return add(10, 20)
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

	// 5. Verify IR was generated
	if ir == "" {
		t.Fatal("Generated IR is empty")
	}

	// 6. Verify IR contains expected elements
	if !strings.Contains(ir, "define i32 @add") {
		t.Error("IR does not contain add function")
	}
	if !strings.Contains(ir, "define i32 @main") {
		t.Error("IR does not contain main function")
	}

	// 7. Write IR to file
	llPath := filepath.Join(tmpDir, "test.ll")
	if err := ioutil.WriteFile(llPath, []byte(ir), 0644); err != nil {
		t.Fatal(err)
	}

	// 8. Compile with clang together with the embedded runtime (provides
	// GC_init/GC_malloc) to verify IR is valid
	runtimePath := filepath.Join(tmpDir, "runtime.c")
	if err := ioutil.WriteFile(runtimePath, runtimelib.Source, 0644); err != nil {
		t.Fatal(err)
	}
	exePath := filepath.Join(tmpDir, "test_exe")
	cmd := exec.Command("clang", "-w", "-Wno-override-module", "-o", exePath, llPath, runtimePath)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clang failed: %s\n%s", err, out)
	}

	// 9. Run executable
	cmd = exec.Command(exePath)
	out, err := cmd.CombinedOutput()

	// 10. Check exit code (should be 30 = 10 + 20)
	if exitError, ok := err.(*exec.ExitError); ok {
		if exitError.ExitCode() != 30 {
			t.Errorf("Expected exit code 30, got %d", exitError.ExitCode())
		}
	} else if err != nil {
		t.Fatalf("Execution failed: %v\nOutput: %s", err, out)
	} else {
		t.Errorf("Expected exit code 30, got 0 (success)")
	}
}

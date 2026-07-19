package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/orktes/orlang/analyser"
	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/codegen/llvm"
	"github.com/orktes/orlang/parser"
	"github.com/orktes/orlang/runtimelib"
)

// compileResult holds the output of a full compilation pipeline.
type compileResult struct {
	Binary      string   // path to the final linked binary
	TempFiles   []string // intermediate files to clean up
	ClangTarget string   // resolved clang target triple flag
}

// linkDirective represents a parsed link statement from source.
type linkDirective struct {
	Kind      ast.LinkKind
	Path      string // the string value from the directive
	SourceDir string // directory of the .or file containing this directive
}

// compileLLVM takes .or source files, generates LLVM IR, compiles to object
// files with clang, links them with the embedded runtime (GC + built-in map),
// and returns the path to the binary.
// outputPath is the desired binary name (empty = derived from first source file).
// Automatically discovers sibling .or files in the same directory.
func compileLLVM(sourceFiles []string, outputPath string) (*compileResult, error) {
	if len(sourceFiles) == 0 {
		return nil, fmt.Errorf("no source files specified")
	}

	result := &compileResult{}
	result.ClangTarget = detectClangTarget()

	// Resolve to absolute path so the binary can be executed without $PATH
	absSource, err := filepath.Abs(sourceFiles[0])
	if err != nil {
		return nil, err
	}
	baseDir := filepath.Dir(absSource)

	// Auto-discover sibling .or files in the same directory
	sourceFiles, err = discoverSourceFiles(sourceFiles, baseDir)
	if err != nil {
		return nil, err
	}

	// Determine output binary name
	if outputPath == "" {
		base := filepath.Base(absSource)
		ext := filepath.Ext(base)
		outputPath = filepath.Join(baseDir, strings.TrimSuffix(base, ext))
	} else {
		outputPath, _ = filepath.Abs(outputPath)
	}
	result.Binary = outputPath

	var objectFiles []string
	var allDirectives []linkDirective

	// 1. Compile each .or file to .ll then to .o, collecting link directives
	for _, srcFile := range sourceFiles {
		llFile, fileNode, err := compileOrToLL(srcFile)
		if err != nil {
			cleanupFiles(objectFiles)
			return nil, fmt.Errorf("compiling %s: %w", srcFile, err)
		}
		result.TempFiles = append(result.TempFiles, llFile)

		// Collect link directives from this source file
		srcDir := filepath.Dir(srcFile)
		if absSrcDir, err := filepath.Abs(srcDir); err == nil {
			srcDir = absSrcDir
		}
		allDirectives = append(allDirectives, extractLinkDirectives(fileNode, srcDir)...)

		oFile, err := compileLLToObj(llFile, result.ClangTarget)
		if err != nil {
			cleanupFiles(objectFiles)
			cleanupFiles(result.TempFiles)
			return nil, fmt.Errorf("assembling %s: %w", llFile, err)
		}
		objectFiles = append(objectFiles, oFile)
		result.TempFiles = append(result.TempFiles, oFile)
	}

	// 2. Gather pkg-config --cflags for C compilation
	cflags := collectPkgConfigCflags(allDirectives)

	// 3. Compile C files from link directives + auto-discovered .c files
	compiledCFiles := make(map[string]bool) // track to avoid duplicates

	// First, compile C files from link "file.c" directives
	for _, dir := range allDirectives {
		if dir.Kind != ast.LinkKindFile {
			continue
		}
		cFile := filepath.Join(dir.SourceDir, dir.Path)
		absC, _ := filepath.Abs(cFile)
		if compiledCFiles[absC] {
			continue
		}
		compiledCFiles[absC] = true

		oFile, err := compileCToObj(cFile, result.ClangTarget, cflags)
		if err != nil {
			cleanupFiles(result.TempFiles)
			return nil, fmt.Errorf("compiling C file %s: %w", cFile, err)
		}
		objectFiles = append(objectFiles, oFile)
		result.TempFiles = append(result.TempFiles, oFile)
	}

	// Also auto-discover any .c files in the base directory (backward compat)
	cFiles, _ := filepath.Glob(filepath.Join(baseDir, "*.c"))
	for _, cFile := range cFiles {
		absC, _ := filepath.Abs(cFile)
		if compiledCFiles[absC] {
			continue
		}
		compiledCFiles[absC] = true

		oFile, err := compileCToObj(cFile, result.ClangTarget, cflags)
		if err != nil {
			cleanupFiles(result.TempFiles)
			return nil, fmt.Errorf("compiling C file %s: %w", cFile, err)
		}
		objectFiles = append(objectFiles, oFile)
		result.TempFiles = append(result.TempFiles, oFile)
	}

	// 3. Link everything with link directives
	if err := linkObjects(objectFiles, outputPath, result.ClangTarget, allDirectives); err != nil {
		cleanupFiles(result.TempFiles)
		return nil, fmt.Errorf("linking: %w", err)
	}

	return result, nil
}

// extractLinkDirectives collects all link statements from an AST file.
func extractLinkDirectives(fileNode *ast.File, sourceDir string) []linkDirective {
	var directives []linkDirective
	for _, node := range fileNode.Body {
		if link, ok := node.(*ast.LinkStatement); ok {
			directives = append(directives, linkDirective{
				Kind:      link.Kind,
				Path:      link.Path.Token.Value.(string),
				SourceDir: sourceDir,
			})
		}
	}
	return directives
}

// compileOrToLL parses, analyses, and generates LLVM IR for a single .or file.
// Returns the .ll file path and the parsed AST (for extracting link directives).
func compileOrToLL(srcFile string) (string, *ast.File, error) {
	file, err := os.Open(srcFile)
	if err != nil {
		return "", nil, err
	}
	defer file.Close()

	fileNode, err := parser.Parse(file)
	if err != nil {
		return "", nil, err
	}

	an, err := analyser.New(fileNode)
	if err != nil {
		return "", nil, err
	}

	basePath := filepath.Dir(srcFile)
	an.FileLoader = func(importPath string) (*ast.File, error) {
		fullPath := filepath.Join(basePath, importPath)
		f, err := os.Open(fullPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return parser.Parse(f)
	}

	fileInfo, err := an.Analyse()
	if err != nil {
		return "", nil, err
	}

	llvmcg := llvm.New(fileInfo)
	ext := filepath.Ext(srcFile)
	baseName := filepath.Base(srcFile)
	moduleName := strings.TrimSuffix(baseName, ext)
	llvmcg.SetModuleName(moduleName)

	code := llvmcg.Generate(fileNode)
	if errs := llvmcg.Errors(); len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = fmt.Sprintf("%s: %s", srcFile, e)
		}
		return "", nil, fmt.Errorf("%s", strings.Join(msgs, "\n"))
	}
	llFile := strings.TrimSuffix(srcFile, ext) + ".ll"
	if err := os.WriteFile(llFile, []byte(code), 0644); err != nil {
		return "", nil, err
	}
	return llFile, fileNode, nil
}

// compileLLToObj compiles an LLVM IR file to a native object file.
func compileLLToObj(llFile string, clangTarget string) (string, error) {
	oFile := strings.TrimSuffix(llFile, ".ll") + ".o"
	args := []string{"-w", "-c", "-o", oFile, llFile}
	if clangTarget != "" {
		args = append([]string{"-target", clangTarget}, args...)
	}
	cmd := exec.Command("clang", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return oFile, nil
}

// compileCToObj compiles a C source file to a native object file.
// extraFlags are additional compiler flags (e.g., pkg-config --cflags output).
func compileCToObj(cFile string, clangTarget string, extraFlags []string) (string, error) {
	oFile := strings.TrimSuffix(cFile, ".c") + ".o"
	args := []string{"-w", "-c", "-o", oFile}
	if clangTarget != "" {
		args = append([]string{"-target", clangTarget}, args...)
	}
	args = append(args, extraFlags...)
	args = append(args, cFile)
	cmd := exec.Command("clang", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return oFile, nil
}

// collectPkgConfigCflags runs pkg-config --cflags for all link pkg directives
// and returns the combined flags for use when compiling C files.
func collectPkgConfigCflags(directives []linkDirective) []string {
	seen := make(map[string]bool)
	var flags []string
	for _, dir := range directives {
		if dir.Kind != ast.LinkKindPkg || seen[dir.Path] {
			continue
		}
		seen[dir.Path] = true
		out, err := exec.Command("pkg-config", "--cflags", dir.Path).Output()
		if err != nil {
			continue
		}
		for _, f := range strings.Fields(strings.TrimSpace(string(out))) {
			flags = append(flags, f)
		}
	}
	return flags
}

// runtimeObject returns the path to the compiled orlang runtime object
// (GC + built-in map), compiling the embedded C source on first use. The
// result is cached in the user cache directory keyed by source hash and
// target, so repeated builds don't recompile it.
func runtimeObject(clangTarget string) (string, error) {
	targetKey := clangTarget
	if targetKey == "" {
		targetKey = "native"
	}
	targetKey = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, targetKey)

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	cacheDir = filepath.Join(cacheDir, "orlang")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return "", fmt.Errorf("creating runtime cache dir: %w", err)
	}

	oFile := filepath.Join(cacheDir, fmt.Sprintf("runtime-%s-%s.o", runtimelib.Hash(), targetKey))
	if _, err := os.Stat(oFile); err == nil {
		return oFile, nil
	}

	cFile := filepath.Join(cacheDir, fmt.Sprintf("runtime-%s.c", runtimelib.Hash()))
	if err := os.WriteFile(cFile, runtimelib.Source, 0644); err != nil {
		return "", fmt.Errorf("writing runtime source: %w", err)
	}
	defer os.Remove(cFile)

	// Compile to a temp name first so a concurrent build never sees a
	// half-written object at the final path.
	tmpO := oFile + ".tmp"
	args := []string{"-O2", "-w", "-c", "-o", tmpO, cFile}
	if clangTarget != "" {
		args = append([]string{"-target", clangTarget}, args...)
	}
	cmd := exec.Command("clang", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		os.Remove(tmpO)
		return "", fmt.Errorf("compiling orlang runtime: %w", err)
	}
	if err := os.Rename(tmpO, oFile); err != nil {
		os.Remove(tmpO)
		return "", fmt.Errorf("installing orlang runtime object: %w", err)
	}
	return oFile, nil
}

// linkObjects links object files into a binary together with the embedded
// orlang runtime and any libraries specified by link directives.
func linkObjects(objectFiles []string, outputPath string, clangTarget string, directives []linkDirective) error {
	runtimeObj, err := runtimeObject(clangTarget)
	if err != nil {
		return err
	}

	args := []string{"-Wno-override-module", "-o", outputPath}
	if clangTarget != "" {
		args = append([]string{"-target", clangTarget}, args...)
	}
	args = append(args, objectFiles...)
	args = append(args, runtimeObj)

	// Process link directives for additional libraries
	seenPkg := map[string]bool{}
	seenLib := map[string]bool{}

	for _, dir := range directives {
		switch dir.Kind {
		case ast.LinkKindPkg:
			if seenPkg[dir.Path] {
				continue
			}
			seenPkg[dir.Path] = true
			pkgFlags, err := exec.Command("pkg-config", "--libs", dir.Path).Output()
			if err != nil {
				return fmt.Errorf("pkg-config --libs %s: %w", dir.Path, err)
			}
			args = append(args, strings.Fields(strings.TrimSpace(string(pkgFlags)))...)
		case ast.LinkKindLib:
			if seenLib[dir.Path] {
				continue
			}
			seenLib[dir.Path] = true
			args = append(args, "-l"+dir.Path)
		}
	}

	if runtime.GOOS == "linux" {
		args = append(args, "-lm")
	}

	cmd := exec.Command("clang", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// detectClangTarget returns the appropriate -target flag for the current platform.
func detectClangTarget() string {
	switch runtime.GOOS {
	case "darwin":
		if runtime.GOARCH == "arm64" {
			return "arm64-apple-macosx14.0"
		}
		return "x86_64-apple-macosx10.15"
	case "linux":
		if runtime.GOARCH == "arm64" {
			return "aarch64-unknown-linux-gnu"
		}
		return "x86_64-unknown-linux-gnu"
	}
	return ""
}

// discoverSourceFiles adds any sibling .or files in baseDir that aren't
// already in the provided list. The explicitly provided files come first.
func discoverSourceFiles(explicit []string, baseDir string) ([]string, error) {
	// Build set of already-included absolute paths
	seen := make(map[string]bool)
	for _, f := range explicit {
		abs, err := filepath.Abs(f)
		if err != nil {
			return nil, err
		}
		seen[abs] = true
	}

	// Discover siblings
	siblings, _ := filepath.Glob(filepath.Join(baseDir, "*.or"))
	result := make([]string, len(explicit))
	copy(result, explicit)
	for _, sib := range siblings {
		abs, err := filepath.Abs(sib)
		if err != nil {
			continue
		}
		if !seen[abs] {
			result = append(result, sib)
			seen[abs] = true
		}
	}
	return result, nil
}

// cleanupFiles removes a list of files, ignoring errors.
func cleanupFiles(files []string) {
	for _, f := range files {
		os.Remove(f)
	}
}

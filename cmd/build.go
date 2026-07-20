package cmd

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/orktes/orlang/codegen/js"
	"github.com/orktes/orlang/codegen/llvm"
	"github.com/spf13/cobra"
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build [files...]",
	Short: "Build Orlang application",
	Long: `Build Orlang application.

By default, compiles to a native binary via LLVM:
  orlang build main.or              # produces ./main binary
  orlang build main.or -o myapp     # produces ./myapp binary

Use --target to emit intermediate formats only (no linking):
  orlang build main.or --target llvm  # produces main.ll
  orlang build main.or --target js    # produces main.js`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("no source files specified")
		}

		target, _ := cmd.Flags().GetString("target")
		output, _ := cmd.Flags().GetString("output")

		switch target {
		case "js":
			return buildJS(args)
		case "llvm":
			// Emit .ll only (backward compatible)
			return buildLLVMIR(args)
		case "":
			// Default: full native binary compilation
			result, err := compileLLVM(args, output)
			if err != nil {
				return err
			}
			// Clean up intermediate files
			cleanupFiles(result.TempFiles)
			fmt.Println(result.Binary)
			return nil
		default:
			return fmt.Errorf("unknown target: %s", target)
		}
	},
}

// buildJS compiles .or files to JavaScript (legacy behavior).
func buildJS(files []string) error {
	for _, filePath := range files {
		fileNode, fileInfo, err := analyseSourceFile(filePath)
		if err != nil {
			return err
		}

		jscg := js.New(fileInfo)
		code := jscg.Generate(fileNode)
		ext := path.Ext(filePath)
		outfile := filePath[0:len(filePath)-len(ext)] + ".js"
		if err := ioutil.WriteFile(outfile, code, 0644); err != nil {
			return err
		}
	}
	return nil
}

// buildLLVMIR compiles .or files to .ll only (no clang/linking).
func buildLLVMIR(files []string) error {
	for _, filePath := range files {
		fileNode, fileInfo, err := analyseSourceFile(filePath)
		if err != nil {
			return err
		}

		llvmcg := llvm.New(fileInfo)
		ext := filepath.Ext(filePath)
		baseName := filepath.Base(filePath)
		moduleName := baseName[0 : len(baseName)-len(ext)]
		llvmcg.SetModuleName(moduleName)
		code := llvmcg.Generate(fileNode)
		if errs := llvmcg.Errors(); len(errs) > 0 {
			for _, e := range errs {
				fmt.Fprintf(os.Stderr, "%s: %s\n", filePath, e)
			}
			return fmt.Errorf("code generation failed with %d error(s)", len(errs))
		}
		outfile := filePath[0:len(filePath)-len(ext)] + ".ll"
		if err := os.WriteFile(outfile, []byte(code), 0644); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	RootCmd.AddCommand(buildCmd)

	buildCmd.Flags().String("target", "", "Emit intermediate format only (llvm, js)")
	buildCmd.Flags().StringP("output", "o", "", "Output binary path")
}

package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [files...] [-- args...]",
	Short: "Build and run Orlang application",
	Long: `Build and run an Orlang application in one step.

Compiles to a native binary via LLVM, executes it, then cleans up.
Arguments after -- are passed to the program:
  orlang run main.or
  orlang run main.or -- arg1 arg2`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return fmt.Errorf("no source files specified")
		}

		// Split args at "--" into source files and program args
		sourceFiles, programArgs := splitArgs(args)

		if len(sourceFiles) == 0 {
			return fmt.Errorf("no source files specified")
		}

		// Build
		result, err := compileLLVM(sourceFiles, "")
		if err != nil {
			return err
		}

		// Always clean up binary and temp files when done
		defer func() {
			cleanupFiles(result.TempFiles)
			os.Remove(result.Binary)
		}()

		// Run the binary, forwarding stdio and signals
		proc := exec.Command(result.Binary, programArgs...)
		proc.Stdin = os.Stdin
		proc.Stdout = os.Stdout
		proc.Stderr = os.Stderr

		// Forward signals to the child process
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			for sig := range sigCh {
				if proc.Process != nil {
					proc.Process.Signal(sig)
				}
			}
		}()

		if err := proc.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			return err
		}

		return nil
	},
}

// splitArgs splits command args at "--" separator.
func splitArgs(args []string) (sourceFiles []string, programArgs []string) {
	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func init() {
	RootCmd.AddCommand(runCmd)
}

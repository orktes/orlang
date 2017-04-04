package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/orktes/orlang/ast"
	"github.com/orktes/orlang/linter"
	"github.com/spf13/cobra"
)

// lintCmd represents the lint command
var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Lint source file",
	Long:  `Lint source file`,
	Run: func(cmd *cobra.Command, args []string) {
		format := cmd.Flag("format").Value.String()

		switch format {
		case "json":
			fmt.Print("[")
		}

		files := args

		for index, filePath := range files {
			file, err := os.Open(filePath)
			if err != nil {
				panic(err)
			}

			lintError, err := linter.Lint(file)
			if err != nil {
				panic(err)
			}

			switch format {
			case "text":
				for _, lintError := range lintError {
					fmt.Println(formatParseError(
						filePath,
						lintError.Position,
						lintError.CodeLine,
						lintError.Message,
					))
				}
			case "json":
				normalizedFilePath, err := filepath.Abs(filePath)
				if err != nil {
					panic(err)
				}
				json.NewEncoder(os.Stdout).Encode(map[string]interface{}{
					"Filename": normalizedFilePath,
					"Errors":   lintError,
				})
			default:
				panic("Unknown output format")
			}

			if index != len(files)-1 {
				switch format {
				case "json":
					fmt.Print(",")
				}
			}
		}

		switch format {
		case "json":
			fmt.Println("]")
		}
	},
}

func formatParseError(filePath string, pos ast.Position, line string, err string) string {
	if line != "" {
		return fmt.Sprintf(
			`%s:%d:%d
----------------------------------------------------------
%s
%s
%s
----------------------------------------------------------`, filePath, pos.Line+1, pos.Column+1, strings.Replace(line, "\t", " ", -1), pointer(pos.Column), pad(pos.Column-int(len(err)/2), err))
	}
	return fmt.Sprintf("%s %#v %s", filePath, pos, err)
}

func pad(padding int, str string) (res string) {
	if padding < 0 {
		padding = 0
	}
	res = strings.Repeat(" ", padding)
	return res + str
}

func pointer(padding int) (res string) {
	return pad(padding, "^")
}

func init() {
	RootCmd.AddCommand(lintCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	lintCmd.PersistentFlags().String("format", "text", "Format for output [text|json]")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// lintCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}

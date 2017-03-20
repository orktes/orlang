package cmd

import (
	"fmt"
	"os"

	"github.com/orktes/orlang/parser/ast"
	"github.com/orktes/orlang/parser/parser"
	"github.com/orktes/orlang/parser/scanner"
	"github.com/orktes/orlang/parser/util"
	"github.com/spf13/cobra"
)

// lintCmd represents the lint command
var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Lint source file",
	Long:  `Lint source file`,
	Run: func(cmd *cobra.Command, args []string) {

		for _, filePath := range args {
			var hr *util.HistoryReader
			file, err := os.Open(filePath)
			if err != nil {
				fmt.Println(hr.FormatError(filePath, err))
				break
			}

			hr = util.NewHistoryReader(file)
			p := parser.NewParser(scanner.NewScanner(hr))
			p.ContinueOnErrors = true
			lastTokenErrorIndex := -2
			p.Error = func(tokenIndx int, pos ast.Position, message string) {
				if tokenIndx != lastTokenErrorIndex+1 {
					fmt.Printf("%s\n\n", hr.FormatParseError(filePath, pos, message))
				}
				lastTokenErrorIndex = tokenIndx
			}
			_, err = p.Parse()

			if err != nil {
				fmt.Printf("%s\n\n", hr.FormatError(filePath, err))
			}
		}
	},
}

func init() {
	RootCmd.AddCommand(lintCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// lintCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// lintCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}

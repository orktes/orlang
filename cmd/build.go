// Copyright © 2017 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"io/ioutil"
	"os"
	"path"

	"github.com/orktes/orlang/ast"

	"github.com/orktes/orlang/analyser"
	"github.com/orktes/orlang/codegen/js"
	"github.com/orktes/orlang/codegen/llvm"
	"github.com/orktes/orlang/parser"
	"github.com/spf13/cobra"
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "Build Orlang application",
	Long:  `Build Orlang application`,
	RunE: func(cmd *cobra.Command, args []string) error {
		files := args

		for _, filePath := range files {
			file, err := os.Open(filePath)
			if err != nil {
				return err
			}

			fileNode, err := parser.Parse(file)
			if err != nil {
				return err
			}

			an, err := analyser.New(fileNode)
			if err != nil {
				return err
			}
			basePath := path.Dir(filePath)
			an.FileLoader = func(importPath string) (*ast.File, error) {
				// Resolve relative to the importing file
				fullPath := path.Join(basePath, importPath)
				f, err := os.Open(fullPath)
				if err != nil {
					return nil, err
				}
				defer f.Close()
				return parser.Parse(f)
			}

			fileInfo, err := an.Analyse()
			if err != nil {
				return err
			}

			target := cmd.Flag("target").Value.String()
			switch target {
			case "js":
				jscg := js.New(fileInfo)
				code := jscg.Generate(fileNode)
				ext := path.Ext(filePath)
				outfile := filePath[0:len(filePath)-len(ext)] + ".js"
				err := ioutil.WriteFile(outfile, code, 0644)
				if err != nil {
					return err
				}
			case "llvm":
				llvmcg := llvm.New(fileInfo)
				// Extract module name from file path (e.g., "lib.or" -> "lib")
				ext := path.Ext(filePath)
				baseName := path.Base(filePath)
				moduleName := baseName[0 : len(baseName)-len(ext)]
				llvmcg.SetModuleName(moduleName)
				code := llvmcg.Generate(fileNode)
				outfile := filePath[0:len(filePath)-len(ext)] + ".ll"
				err := ioutil.WriteFile(outfile, []byte(code), 0644)
				if err != nil {
					return err
				}
			}
		}

		return nil
	},
}

func init() {
	RootCmd.AddCommand(buildCmd)

	buildCmd.PersistentFlags().String("target", "js", "Target platform")
	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// buildCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// buildCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

}

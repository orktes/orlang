// Package stdlib embeds the orlang standard library modules that ship
// inside the compiler binary. Programs import them with paths under the
// std/ prefix, e.g.:
//
//	import { server } from "std/http.or"
//
// The build pipeline resolves these imports from the embedded sources and
// compiles each module alongside the program, so no files need to exist
// on disk.
package stdlib

import (
	"embed"
	"strings"
)

//go:embed std/*.or
var std embed.FS

// IsStdPath reports whether an import path refers to an embedded standard
// library module.
func IsStdPath(path string) bool {
	return strings.HasPrefix(path, "std/")
}

// Load returns the source of an embedded standard library module by its
// import path (e.g. "std/http.or").
func Load(path string) ([]byte, bool) {
	if !IsStdPath(path) {
		return nil, false
	}
	data, err := std.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

// Modules lists the available standard library import paths.
func Modules() []string {
	entries, err := std.ReadDir("std")
	if err != nil {
		return nil
	}
	var paths []string
	for _, e := range entries {
		paths = append(paths, "std/"+e.Name())
	}
	return paths
}

// Package runtimelib embeds the C runtime that is compiled and linked into
// every orlang executable: a conservative garbage collector, the built-in
// map implementation, and the HTTP and JSON runtimes backing the standard
// library. Because the sources ship inside the compiler binary, produced
// executables have no external runtime dependencies.
package runtimelib

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"sort"
)

//go:embed csrc/*.c
var csrc embed.FS

// Source is the core runtime (GC + map), kept for tests and tools that
// need a single translation unit providing GC_malloc/GC_init.
var Source []byte

// Sources maps file names (e.g. "runtime.c") to their embedded contents,
// in deterministic order via SourceNames.
var Sources = map[string][]byte{}

// SourceNames lists the embedded runtime files in sorted order.
var SourceNames []string

func init() {
	entries, err := csrc.ReadDir("csrc")
	if err != nil {
		panic(err)
	}
	for _, e := range entries {
		data, err := csrc.ReadFile("csrc/" + e.Name())
		if err != nil {
			panic(err)
		}
		Sources[e.Name()] = data
		SourceNames = append(SourceNames, e.Name())
	}
	sort.Strings(SourceNames)
	Source = Sources["runtime.c"]
}

// Hash returns a short content hash over all runtime sources, used to key
// the compiled-object cache so stale objects are never reused.
func Hash() string {
	sum := sha256.New()
	for _, name := range SourceNames {
		sum.Write([]byte(name))
		sum.Write(Sources[name])
	}
	return hex.EncodeToString(sum.Sum(nil))[:16]
}

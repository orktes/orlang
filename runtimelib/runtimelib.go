// Package runtimelib embeds the C runtime that is compiled and linked into
// every orlang executable: a conservative garbage collector and the built-in
// map implementation. Because the source ships inside the compiler binary,
// produced executables have no external runtime dependencies.
package runtimelib

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
)

//go:embed csrc/runtime.c
var Source []byte

// Hash returns a short content hash of the runtime source, used to key the
// compiled-object cache so stale objects are never reused across versions.
func Hash() string {
	sum := sha256.Sum256(Source)
	return hex.EncodeToString(sum[:])[:16]
}

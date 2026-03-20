#!/bin/bash
set -e

# Detect real hardware architecture (works even under Rosetta)
if sysctl -n hw.optional.arm64 2>/dev/null | grep -q 1; then
  CLANG_TARGET="-target arm64-apple-macosx14.0"
elif [ "$(uname -m)" = "x86_64" ]; then
  CLANG_TARGET="-target x86_64-apple-macosx10.15"
else
  CLANG_TARGET=""
fi

# Build the orlang compiler
(cd ../../ && go install .)

echo "=== Compiling routes.or ==="
orlang build routes.or --target llvm
clang $CLANG_TARGET -w -c -o routes.o routes.ll
rm routes.ll

echo "=== Compiling main.or ==="
orlang build main.or --target llvm
clang $CLANG_TARGET -w -c -o main.o main.ll
rm main.ll

echo "=== Compiling http.c ==="
clang $CLANG_TARGET -w -c -o http.o http.c $(pkg-config --cflags libmicrohttpd)

echo "=== Linking ==="
clang $CLANG_TARGET -Wno-override-module -o server main.o routes.o http.o \
    $(pkg-config --libs libmicrohttpd) $(pkg-config --libs bdw-gc)

rm -f main.o routes.o http.o
echo "=== Build complete: ./server ==="

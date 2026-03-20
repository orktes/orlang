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

(cd ../ && go install .)

function run_test {
  dir=$1
  echo "Running test $dir"
  pushd $dir
    echo "Linting $dir"
    orlang lint main.or
    echo "Building $dir"

    # Check if there are any .or files other than main.or (for imports)
    shopt -s nullglob
    or_files=(*.or)
    shopt -u nullglob

    # Compile all .or files to .ll
    object_files=()
    for or_file in "${or_files[@]}"; do
      if [ "$or_file" != "main.or" ]; then
        echo "Compiling $or_file"
        orlang build "$or_file" --target llvm
        ll_file="${or_file%.or}.ll"
        o_file="${or_file%.or}.o"
        clang $CLANG_TARGET -w -c -o "$o_file" "$ll_file"
        object_files+=("$o_file")
        rm "$ll_file"
      fi
    done

    # Compile any C runtime files if they exist
    shopt -s nullglob  # Make globs expand to nothing if no matches
    for c_file in *.c; do
      echo "Compiling C runtime: $c_file"
      clang $CLANG_TARGET -w -c -o "${c_file%.c}.o" "$c_file"
      object_files+=("${c_file%.c}.o")
    done
    shopt -u nullglob  # Reset to default behavior

    # Compile main.or
    orlang build main.or --target llvm
    clang $CLANG_TARGET -w -c -o main.o main.ll

    # Link all object files
    echo "Linking"
    clang $CLANG_TARGET -Wno-override-module -o main main.o "${object_files[@]}" $(pkg-config --libs bdw-gc)
    rm main.o "${object_files[@]}"

    echo "Running"
    if [ -f args.txt ]; then
      output="$(./main "$(<args.txt)" 2>&1 | tee /dev/stderr)"
    else
      output="$(./main 2>&1 | tee /dev/stderr)"
    fi

    expected=$(cat expected.txt)

    if [ "$output" = "$expected" ];
    then
      echo "Test produced expected result"
      rm main.ll
      rm main
    else
      echo "Invalid output"
      echo "Output:"
      echo "$output"
      echo "Expected:"
      echo "$expected"
      echo "Leaving main.ll and main for debugging"
      exit 1
    fi
  popd
}


if [ -z "$1" ]; then
  for dir in */; do
    # Skip if dir ends with _skip
    if [[ $dir == */_skip/* ]]; then
      continue
    fi
    run_test $dir
  done
else
  run_test $1
fi

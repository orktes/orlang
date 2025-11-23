#!/bin/bash

set -e




(cd ../ && go install .)

function run_test {
  dir=$1
  echo "Running test $dir"
  pushd $dir
    echo "Linting"
    orlang lint main.or
    echo "Building"
    
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
        clang -c -o "$o_file" "$ll_file"
        object_files+=("$o_file")
        rm "$ll_file"
      fi
    done
    
    # Compile main.or
    orlang build main.or --target llvm
    clang -c -o main.o main.ll
    rm main.ll
    
    # Link all object files
    clang -Wno-override-module -o main main.o "${object_files[@]}"
    rm main.o "${object_files[@]}"
    
    if [ -f args.txt ]; then
      output="$(./main \"$(<args.txt)\")"
    else
      output="$(./main)"
    fi
    rm ./main
    if [ "$output" = "$(cat expected.txt)" ];
    then
      echo "Test produced expected result"
    else
      echo "Invalid output $output"
      exit 1
    fi
  popd
}


if [ -z "$1" ]; then  
  for dir in */; do
    # Skip if dir ends with _skip
    if [[ $dir == */_skip/ ]]; then
      continue
    fi
    run_test $dir
  done
else
  run_test $1
fi

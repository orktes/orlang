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
    orlang build main.or --target llvm
    clang -Wno-override-module -o main main.ll
    rm main.ll
    if [ -f args.txt ]; then
      output="$(./main \"$(< args.txt)\")"
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

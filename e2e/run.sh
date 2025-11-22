#!/bin/bash

set -e

(cd ../ && go install .)

for dir in */; do
  echo "Running test $dir"
  pushd $dir
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
done

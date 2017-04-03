#!/bin/bash

set -e

for dir in */; do
  echo "Running test $dir"
  pushd $dir
    orlang build main.or -o main
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

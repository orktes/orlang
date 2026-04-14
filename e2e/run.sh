#!/bin/bash

set -e

(cd ../ && go install .)

function run_test {
  dir=$1
  echo "Running test $dir"
  pushd $dir
    echo "Linting $dir"
    orlang lint main.or

    echo "Building $dir"
    orlang build main.or

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
      rm main
    else
      echo "Invalid output"
      echo "Output:"
      echo "$output"
      echo "Expected:"
      echo "$expected"
      echo "Leaving main for debugging"
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

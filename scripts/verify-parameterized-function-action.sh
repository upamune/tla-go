#!/usr/bin/env bash
set -eu

if [ "$#" -ne 1 ]; then
  printf 'Usage: %s /path/to/tla2tools.jar\n' "$0" >&2
  exit 2
fi
jar_path=$(cd "$(dirname "$1")" && pwd)/$(basename "$1")
repo_root=$(cd "$(dirname "$0")/.." && pwd)
task_dir=$(mktemp -d)
trap 'rm -rf "$task_dir"' EXIT
cp "$repo_root"/checker/testdata/tlc-parameterized-function-action/* "$task_dir"/
cd "$task_dir"

java -cp "$jar_path" tlc2.TLC -config Success.cfg M.tla > success.log 2>&1
rg -q 'Model checking completed\. No error has been found' success.log
rg -q '7 states generated, 5 distinct states found' success.log
rg -q 'depth of the complete state graph search is 3' success.log

if java -cp "$jar_path" tlc2.TLC -config Violation.cfg M.tla > violation.log 2>&1; then
  status=0
else
  status=$?
fi
test "$status" -eq 12
rg -q 'Invariant Inv is violated' violation.log
rg -q '5 states generated, 4 distinct states found' violation.log
rg -q 'depth of the complete state graph search is 3' violation.log
states=$(awk '/^x = / {printf "%s ", $3}' violation.log)
test "$states" = '0 1 3 '
printf 'TLC parameterized function-action checks passed.\n'

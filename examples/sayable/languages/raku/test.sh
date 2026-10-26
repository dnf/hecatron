#!/usr/bin/env bash
# Run each of one library's Raku test files.
set -euo pipefail

for _file in t/*.rakutest; do
    raku -Ilib "${_file}"
done

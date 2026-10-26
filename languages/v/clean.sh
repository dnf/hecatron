#!/usr/bin/env bash
# Remove one library's V build outputs.
set -euo pipefail

rm -rf "sayable${1}" "sayable${1}.dylib" "sayable${1}.so" "sayable${1}.dll" v

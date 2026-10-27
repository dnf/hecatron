# Smoke test for kotlin: compile a small Kotlin program with kotlinc into a
# jar and run it on the JVM, in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'fun main() {\n    println((1..9).sum())\n}\n' > Hello.kt
kotlinc Hello.kt -include-runtime -d hello.jar 2>&1 | grep -v '^warning: unable to find kotlin-stdlib' || true
_out="$(java -jar hello.jar)"
[ "${_out}" = "45" ] || { echo "expected 45, got: ${_out}" >&2; exit 1; }
echo "${_out}"

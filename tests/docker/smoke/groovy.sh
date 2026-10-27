# Smoke test for groovy: compile a small Groovy program with groovyc, run it
# on the JVM, and run a script directly with groovy, in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'class Hello {\n    static void main(String[] args) {\n        println((1..9).sum())\n    }\n}\n' > Hello.groovy
groovyc Hello.groovy
_out="$(groovy Hello.groovy)"
[ "${_out}" = "45" ] || { echo "expected 45, got: ${_out}" >&2; exit 1; }
_script="$(groovy -e 'println "hello"')"
[ "${_script}" = "hello" ] || { echo "expected hello, got: ${_script}" >&2; exit 1; }
echo "${_out}"

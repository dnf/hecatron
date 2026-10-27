# Smoke test for scala: compile a small Scala 3 program with scalac and run
# it with scala, in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf '@main def hello(): Unit =\n  println((1 to 9).sum)\n' > Hello.scala
mkdir out && scalac -d out Hello.scala
_out="$(scala -classpath out hello)"
[ "${_out}" = "45" ] || { echo "expected 45, got: ${_out}" >&2; exit 1; }
echo "${_out}"

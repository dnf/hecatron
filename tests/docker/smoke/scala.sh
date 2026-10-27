# Smoke test for scala: compile a small Scala 3 program with scalac and run
# it with scala, in a scratch directory. Since Scala 3.5 (Homebrew's scala)
# the scala command is Scala CLI, which takes the main class as -M; Linux's
# LTS 3.3 has the classic runner, which takes it as an argument.
# shellcheck shell=bash
set -euo pipefail
printf '@main def hello(): Unit =\n  println((1 to 9).sum)\n' > Hello.scala
mkdir out && scalac -d out Hello.scala
if scala --version 2>&1 | grep -q '^Scala version (default)'; then
    _out="$(scala --server=false -classpath out -M hello)"
else
    _out="$(scala -classpath out hello)"
fi
[ "${_out}" = "45" ] || { echo "expected 45, got: ${_out}" >&2; exit 1; }
echo "${_out}"

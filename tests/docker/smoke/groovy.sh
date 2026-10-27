# Smoke test for groovy: the installer provides only the JDK (Groovy itself
# comes from a project's Gradle or Maven build), so build and run a Java
# program with it in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'class Hello { public static void main(String[] a) { System.out.println(java.util.stream.IntStream.rangeClosed(1, 9).sum()); } }\n' > Hello.java
javac Hello.java
_out="$(java Hello)"
[ "${_out}" = "45" ] || { echo "expected 45, got: ${_out}" >&2; exit 1; }
echo "${_out}"

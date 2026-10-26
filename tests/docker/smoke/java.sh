# Smoke test for java: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'class Hello { public static void main(String[] a) { System.out.println("hello"); } }\n' > Hello.java
javac Hello.java && java Hello

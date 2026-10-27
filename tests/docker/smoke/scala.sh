# Smoke test for scala: the installer provides the JDK (scalac and sbt come
# only with a component's build.sbt), so build and run a hello world on it.
# shellcheck shell=bash
set -euo pipefail
printf 'class Hello { public static void main(String[] a) { System.out.println(6 * 7); } }\n' > Hello.java
javac Hello.java
[ "$(java Hello)" = "42" ]

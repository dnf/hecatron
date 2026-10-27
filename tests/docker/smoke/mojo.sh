# Smoke test for mojo: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'def main():\n    print(6 * 7)\n' > hello.mojo
[ "$(mojo run hello.mojo)" = "42" ]
mkdir formatted && cp hello.mojo formatted/ && mojo format --quiet formatted
diff -u hello.mojo formatted/hello.mojo

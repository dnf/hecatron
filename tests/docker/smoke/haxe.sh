# Smoke test for haxe: build and run a small program in a scratch directory,
# on Haxe's interpreter, and check that haxelib starts.
# shellcheck shell=bash
set -euo pipefail
printf 'class Main {\n\tstatic function main() {\n\t\ttrace("hello");\n\t}\n}\n' > Main.hx
haxe --main Main --interp
haxelib version

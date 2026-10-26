# Smoke test for c: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf '#include <stdio.h>\nint main(void) { puts("hello"); return 0; }\n' > hello.c
cc -Wall -Werror -o hello hello.c && ./hello
cmake --version | head -1

# Smoke test for javascript: run a hello world with Node.js in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
mkdir hello && cd hello && npm init -y >/dev/null
printf 'const greeting = "hello";\nconsole.log(greeting);\n' > hello.js
node hello.js

# Smoke test for typescript: build and run a small program in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
mkdir hello && cd hello && npm init -y >/dev/null && npm install --silent typescript
printf 'const greeting: string = "hello";\nconsole.log(greeting);\n' > hello.ts
npx tsc --strict hello.ts && node hello.js

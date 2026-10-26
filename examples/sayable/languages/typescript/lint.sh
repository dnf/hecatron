#!/usr/bin/env bash
# Typecheck and lint one TypeScript library ($1: age, date, duration or time).
set -euo pipefail
[ -d node_modules ] || npm install
npm run lint

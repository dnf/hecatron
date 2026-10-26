#!/usr/bin/env bash
# Lint one Lua library ($1: age, date, duration or time).
set -euo pipefail
luacheck "sayable${1}.lua" "test_sayable${1}.lua" locales/

#!/usr/bin/env bash
# Compile one library's D sources to an object file with ldc2 (names differ per library).
set -euo pipefail

ldc2 -I=source "source/sayable${1}/locale.d" "source/sayable${1}/format_${1}.d" "source/sayable${1}/locales.d" -of="sayable${1}.o" -c

#!/usr/bin/env bash
# Compile one library's V module (a shared library, since the module has no main).
set -euo pipefail

v -shared -o "sayable${1}" locales

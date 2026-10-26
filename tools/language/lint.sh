#!/usr/bin/env bash
# `just lint <lang>`: run .hecatron/languages/<lang>/lint.sh for every library.
# Usage: lint.sh <lang> [library ...]
exec bash "$(dirname "$0")/each-library.sh" lint "$@"

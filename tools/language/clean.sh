#!/usr/bin/env bash
# `just clean <lang>`: run .hecatron/languages/<lang>/clean.sh for every library.
# Usage: clean.sh <lang> [library ...]
exec bash "$(dirname "$0")/each-library.sh" clean "$@"

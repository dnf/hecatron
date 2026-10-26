#!/usr/bin/env bash
# `just build <lang>`: run .hecatron/languages/<lang>/build.sh for every library.
# Usage: build.sh <lang> [library ...]
exec bash "$(dirname "$0")/each-library.sh" build "$@"

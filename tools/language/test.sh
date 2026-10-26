#!/usr/bin/env bash
# `just test <lang>`: run .hecatron/languages/<lang>/test.sh for every library.
# Usage: test.sh <lang> [library ...]
exec bash "$(dirname "$0")/each-library.sh" test "$@"

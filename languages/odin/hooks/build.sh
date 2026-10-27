#!/usr/bin/env bash
# Default build hook: the package, as an executable when it has a main procedure, else as a static library (both named after the directory, as Odin does).
source "$(dirname "$0")/../../_hooks.sh"
exists '*.odin' || unknown "no .odin files"
if grep -Eq '^main[[:space:]]*::' ./*.odin; then
    run odin build .
else
    run odin build . -build-mode:lib
fi

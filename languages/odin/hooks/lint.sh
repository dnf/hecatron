#!/usr/bin/env bash
# Default lint hook: the compiler's vet checks (unused variables and imports, shadowing...) and Odin's style rules.
source "$(dirname "$0")/../../_hooks.sh"
exists '*.odin' || unknown "no .odin files"
run odin check . -vet -strict-style -no-entry-point

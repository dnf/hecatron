#!/usr/bin/env bash
# Default clean hook: the compiled output in build/, keeping the downloaded
# packages (build/packages) that `gleam clean` would also delete.
source "$(dirname "$0")/../../_hooks.sh"
[ -f gleam.toml ] || unknown "no gleam.toml"
run rm -rf build/dev build/prod build/lsp

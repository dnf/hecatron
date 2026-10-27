#!/usr/bin/env bash
# Default test hook: the package's tests (test/runtests.jl) through Pkg.
source "$(dirname "$0")/../../_hooks.sh"
[ -f Project.toml ] || unknown "no Project.toml"
run julia --project=. -e 'using Pkg; Pkg.test()'

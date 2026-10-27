#!/usr/bin/env bash
# Default build hook: install the project's dependencies, run its
# deps/build.jl if any, and precompile it.
source "$(dirname "$0")/../../_hooks.sh"
[ -f Project.toml ] || unknown "no Project.toml"
if [ -f deps/build.jl ]; then
    run julia --project=. -e 'using Pkg; Pkg.instantiate(); Pkg.build(); Pkg.precompile()'
else
    run julia --project=. -e 'using Pkg; Pkg.instantiate(); Pkg.precompile()'
fi

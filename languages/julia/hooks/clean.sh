#!/usr/bin/env bash
# Default clean hook: what a package's build and documentation leave in it
# (deps/build.log, Documenter's docs/build/). Precompiled code lives in the
# Julia depot, shared with other projects, so it stays.
source "$(dirname "$0")/../../_hooks.sh"
[ -f Project.toml ] || unknown "no Project.toml"
run rm -rf deps/build.log docs/build

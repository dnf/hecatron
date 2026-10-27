#!/usr/bin/env bash
# Default build hook: build the rock from its rockspec into a binary .rock
# in this directory, without installing it.
source "$(dirname "$0")/../../_hooks.sh"
exists '*.rockspec' || skip "no rockspec, and Lua sources need no build"
run luarocks make --pack-binary-rock

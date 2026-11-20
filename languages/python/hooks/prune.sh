#!/usr/bin/env bash
# Default prune hook: with --caches, uv's own trim of its cache. A project's
# virtualenv is its installed dependencies, which prune leaves.
source "$(dirname "$0")/../../_hooks.sh"
prune_caches || skip "nothing in a Python project to prune (hecatron clean python removes the build's caches); --caches trims uv's cache"
has uv || skip "uv isn't installed, and pip can't trim its cache (pip cache purge empties it)"
prune_py tool "$(uv cache dir)" -- uv cache prune

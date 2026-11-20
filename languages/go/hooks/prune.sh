#!/usr/bin/env bash
# Default prune hook: nothing to do. The go command trims its build cache
# itself, and the module cache is the dependencies' sources.
source "$(dirname "$0")/../../_hooks.sh"
skip "the go command trims its build cache (GOCACHE) itself, removing entries unused for 5 days; the module cache is the dependencies' sources (go clean -cache and -modcache remove them whole)"

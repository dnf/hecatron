#!/usr/bin/env bash
# Default deps hook: swift package resolve, held to Package.resolved (see deps_swift in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_swift

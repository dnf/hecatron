#!/usr/bin/env bash
# Default deps hook: mix deps.get (see deps_elixir in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_elixir

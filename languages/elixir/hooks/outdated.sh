#!/usr/bin/env bash
# Default outdated hook: mix hex.outdated (see outdated_elixir in languages/_outdated.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_outdated.sh"
outdated_elixir

#!/usr/bin/env bash
# Default format hook: mix format (see format_elixir in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_elixir

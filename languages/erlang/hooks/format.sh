#!/usr/bin/env bash
# Default format hook: erlfmt, where rebar.config has it (see format_erlang in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_erlang

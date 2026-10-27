#!/usr/bin/env bash
# Default format hook: StyLua, where configured (see format_lua in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_lua

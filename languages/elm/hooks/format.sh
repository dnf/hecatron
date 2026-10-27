#!/usr/bin/env bash
# Default format hook: elm-format (see format_elm in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_elm

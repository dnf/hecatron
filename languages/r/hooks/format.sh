#!/usr/bin/env bash
# Default format hook: Air, where configured (see format_r in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_r

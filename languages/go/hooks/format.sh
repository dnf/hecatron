#!/usr/bin/env bash
# Default format hook: gofmt (see format_go in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_go

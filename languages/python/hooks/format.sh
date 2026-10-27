#!/usr/bin/env bash
# Default format hook: Ruff's formatter, or Black where configured (see format_python in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_python

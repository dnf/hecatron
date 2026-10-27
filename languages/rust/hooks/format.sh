#!/usr/bin/env bash
# Default format hook: rustfmt through Cargo (see format_rust in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_rust

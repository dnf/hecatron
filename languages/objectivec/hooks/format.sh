#!/usr/bin/env bash
# Default format hook: clang-format, where the project has a .clang-format (see
# format_clang in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_clang m mm h

#!/usr/bin/env bash
# Default format hook: dart format (see format_dart in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_dart

#!/usr/bin/env bash
# Default format hook: perltidy, where configured (see format_perl in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_perl

#!/usr/bin/env bash
# Default format hook: RuboCop's layout cops, or Standard (see format_ruby in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_ruby

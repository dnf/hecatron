#!/usr/bin/env bash
# Default format hook: PHP CS Fixer or Laravel Pint (see format_php in languages/_format.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_format.sh"
format_php

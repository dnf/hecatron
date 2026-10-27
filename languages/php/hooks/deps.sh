#!/usr/bin/env bash
# Default deps hook: composer install, from composer.lock (see deps_php in languages/_deps.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_deps.sh"
deps_php

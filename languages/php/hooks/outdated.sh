#!/usr/bin/env bash
# Default outdated hook: composer outdated (see outdated_php in languages/_outdated.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_outdated.sh"
outdated_php

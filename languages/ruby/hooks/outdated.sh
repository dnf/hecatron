#!/usr/bin/env bash
# Default outdated hook: bundle outdated (see outdated_ruby in languages/_outdated.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_outdated.sh"
outdated_ruby

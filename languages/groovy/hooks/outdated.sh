#!/usr/bin/env bash
# Default outdated hook: Maven's versions plugin, or Gradle's Versions plugin (see outdated_jvm in languages/_outdated.sh).
source "$(dirname "$0")/../../_hooks.sh"
source "$(dirname "$0")/../../_outdated.sh"
outdated_jvm

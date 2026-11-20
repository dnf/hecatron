#!/usr/bin/env bash
# Default prune hook: a Gradle build's project caches for the Gradle
# versions it no longer uses (prune_gradle in languages/_hooks.sh). Gradle
# trims ~/.gradle itself.
source "$(dirname "$0")/../../_hooks.sh"
prune_gradle

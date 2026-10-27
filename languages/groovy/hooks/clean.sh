#!/usr/bin/env bash
# Default clean hook: Maven, Gradle or sbt, whichever the directory uses (see jvm_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
jvm_hook clean

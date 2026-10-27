#!/usr/bin/env bash
# Default deps hook: Maven's dependency:go-offline, Gradle's dependency report, or sbt update
# (see jvm_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
jvm_hook deps

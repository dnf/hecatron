#!/usr/bin/env bash
# Default format hook: the formatter plugin the Maven, Gradle or sbt build
# configures (see jvm_format_* in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
jvm_hook format

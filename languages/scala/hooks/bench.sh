#!/usr/bin/env bash
# Default bench hook: JMH or kotlinx-benchmark through Gradle's plugin, sbt-jmh through sbt.
source "$(dirname "$0")/../../_hooks.sh"
jvm_hook bench

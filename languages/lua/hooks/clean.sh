#!/usr/bin/env bash
# Default clean hook: the binary rocks the build packs, and LuaCov's
# coverage reports.
source "$(dirname "$0")/../../_hooks.sh"
run rm -f ./*.rock luacov.stats.out luacov.report.out

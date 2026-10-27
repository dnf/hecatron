#!/usr/bin/env bash
# Default test hook: compile and run test.hxml (or tests.hxml) (see haxe_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
haxe_hook test

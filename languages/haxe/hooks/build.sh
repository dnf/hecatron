#!/usr/bin/env bash
# Default build hook: compile build.hxml, else every other .hxml (see haxe_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
haxe_hook build

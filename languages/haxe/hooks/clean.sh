#!/usr/bin/env bash
# Default clean hook: remove the outputs the .hxml files name (see haxe_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
haxe_hook clean

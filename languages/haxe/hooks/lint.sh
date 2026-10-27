#!/usr/bin/env bash
# Default lint hook: Haxe has no standard linter (see haxe_hook in languages/_hooks.sh).
source "$(dirname "$0")/../../_hooks.sh"
haxe_hook lint

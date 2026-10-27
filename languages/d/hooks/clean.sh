#!/usr/bin/env bash
# Default clean hook: dub (its build cache and outputs).
source "$(dirname "$0")/../../_hooks.sh"
{ [ -f dub.json ] || [ -f dub.sdl ]; } || unknown "no dub.json or dub.sdl"
run dub clean

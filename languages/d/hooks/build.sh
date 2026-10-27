#!/usr/bin/env bash
# Default build hook: dub.
source "$(dirname "$0")/../../_hooks.sh"
{ [ -f dub.json ] || [ -f dub.sdl ]; } || unknown "no dub.json or dub.sdl"
run dub build

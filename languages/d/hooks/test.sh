#!/usr/bin/env bash
# Default test hook: dub (the package'\''s unittest blocks).
source "$(dirname "$0")/../../_hooks.sh"
{ [ -f dub.json ] || [ -f dub.sdl ]; } || unknown "no dub.json or dub.sdl"
run dub test

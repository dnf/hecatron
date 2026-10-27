#!/usr/bin/env bash
# Default lint hook: D-Scanner through dub, with the package'\''s own dscanner.ini if any.
source "$(dirname "$0")/../../_hooks.sh"
{ [ -f dub.json ] || [ -f dub.sdl ]; } || unknown "no dub.json or dub.sdl"
run dub lint

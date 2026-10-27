#!/usr/bin/env bash
# Default lint hook: luacheck, with the project's .luacheckrc, else allowing
# busted's globals (describe, it...) that specs use.
source "$(dirname "$0")/../../_hooks.sh"
if [ -f .luacheckrc ]; then
    run luacheck .
else
    run luacheck --std max+busted .
fi

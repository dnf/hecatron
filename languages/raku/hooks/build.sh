#!/usr/bin/env bash
# Default build hook: zef build, for a distribution with a Build.rakumod (or
# Build.pm6); others have nothing to build.
source "$(dirname "$0")/../../_hooks.sh"
if [ -f META6.json ] && { [ -f Build.rakumod ] || [ -f Build.pm6 ] || [ -f Build.pm ]; }; then
    run zef build .
else
    skip "no Build.rakumod, and Raku modules need no build"
fi

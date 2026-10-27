#!/usr/bin/env bash
# Default lint hook: RuboCop, or Standard, whichever the bundle has.
source "$(dirname "$0")/../../_hooks.sh"
bundle_deps
if gem_listed rubocop; then
    ruby_run rubocop
elif gem_listed standard; then
    ruby_run standardrb
else
    skip "neither RuboCop nor Standard is in the bundle"
fi

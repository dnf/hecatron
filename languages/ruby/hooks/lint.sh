#!/usr/bin/env bash
# Default lint hook: RuboCop, or Standard, whichever the bundle has.
# shellcheck source=languages/_hooks.sh
source "$(dirname "$0")/../../_hooks.sh"
ruby_project
bundle_deps
if gem_listed rubocop; then
    checks_format
    ruby_run rubocop
elif gem_listed standard; then
    checks_format
    ruby_run standardrb
else
    skip "neither RuboCop nor Standard is in the bundle"
fi

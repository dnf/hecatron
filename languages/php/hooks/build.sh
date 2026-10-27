#!/usr/bin/env bash
# Default build hook: composer.json's build script; a PHP library otherwise
# has nothing to build.
source "$(dirname "$0")/../../_hooks.sh"
[ -f composer.json ] || unknown "no composer.json"
composer_script build || skip "no build script in composer.json"
composer_deps
run composer run-script build

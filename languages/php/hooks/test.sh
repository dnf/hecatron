#!/usr/bin/env bash
# Default test hook: composer.json's test script, else PHPUnit.
source "$(dirname "$0")/../../_hooks.sh"
[ -f composer.json ] || unknown "no composer.json"
composer_deps
if composer_script test; then
    run composer run-script test
elif [ -x vendor/bin/phpunit ]; then
    run vendor/bin/phpunit
else
    unknown "no test script in composer.json and no PHPUnit in vendor/bin"
fi

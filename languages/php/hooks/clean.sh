#!/usr/bin/env bash
# Default clean hook: composer.json's clean script, else PHPUnit's and the
# linters' caches (vendor/ is installed dependencies, so it stays).
source "$(dirname "$0")/../../_hooks.sh"
[ -f composer.json ] || unknown "no composer.json"
if composer_script clean; then
    run composer run-script clean
else
    run rm -rf .phpunit.cache .phpunit.result.cache .php-cs-fixer.cache .phpcs-cache
fi

#!/usr/bin/env bash
# Default lint hook: composer.json's lint script, else each of PHPStan,
# PHP_CodeSniffer and PHP CS Fixer that the project installs and configures.
source "$(dirname "$0")/../../_hooks.sh"
[ -f composer.json ] || unknown "no composer.json"
composer_deps
if composer_script lint; then
    run composer run-script lint
    exit 0
fi
_linted=0
if [ -x vendor/bin/phpstan ] && exists 'phpstan.neon phpstan.neon.dist phpstan.dist.neon'; then
    run vendor/bin/phpstan analyse --no-progress
    _linted=1
fi
if [ -x vendor/bin/phpcs ] && exists 'phpcs.xml phpcs.xml.dist .phpcs.xml .phpcs.xml.dist'; then
    run vendor/bin/phpcs
    _linted=1
fi
if [ -x vendor/bin/php-cs-fixer ] && exists '.php-cs-fixer.php .php-cs-fixer.dist.php'; then
    run vendor/bin/php-cs-fixer fix --dry-run --diff
    _linted=1
fi
[ "${_linted}" -eq 1 ] || skip "no lint script in composer.json and no configured PHPStan, PHP_CodeSniffer or PHP CS Fixer"

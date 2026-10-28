#!/usr/bin/env bash
# Default test hook: composer.json's test script, else PHPUnit. Without a
# composer.json here, a PHPUnit configuration here runs on the PHPUnit of the
# nearest composer.json above (Avro's lang/php, whose dependencies are the
# root's).
source "$(dirname "$0")/../../_hooks.sh"
if [ ! -f composer.json ]; then
    _composer_dir="$(dir_above_with composer.json)" || unknown "no composer.json here or above it in the project"
    exists 'phpunit.xml phpunit.xml.dist phpunit.dist.xml' ||
        unknown "no composer.json here, and no PHPUnit configuration for ${_composer_dir}'s"
    echo "php: test: the dependencies for $(pwd) are in ${_composer_dir}"
    (cd "${_composer_dir}" && composer_deps)
    [ -x "${_composer_dir}/vendor/bin/phpunit" ] || unknown "no PHPUnit in ${_composer_dir}/vendor/bin"
    run "${_composer_dir}/vendor/bin/phpunit"
    exit
fi
composer_deps
if composer_script test; then
    run composer run-script test
elif [ -x vendor/bin/phpunit ]; then
    run vendor/bin/phpunit
else
    unknown "no test script in composer.json and no PHPUnit in vendor/bin"
fi

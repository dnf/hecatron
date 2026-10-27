# Smoke test for php: run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf '<?php\necho 6 * 7, "\\n";\n' > hello.php
php -l hello.php
[ "$(php hello.php)" = "42" ]
composer --version

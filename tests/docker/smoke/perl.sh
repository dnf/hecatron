# Smoke test for perl: run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
printf 'use strict;\nuse warnings;\nprint 6 * 7, "\\n";\n' > hello.pl
perl -c hello.pl
[ "$(perl hello.pl)" = "42" ]

#!/usr/bin/env bash
# Default lint hook: lintr, where the project configures it with a .lintr
# file, failing on any lint.
source "$(dirname "$0")/../../_hooks.sh"
[ -f .lintr ] || skip "no .lintr configuration"
if [ -f DESCRIPTION ]; then
    run Rscript -e 'lints <- lintr::lint_package(); print(lints); quit(status = length(lints) > 0)'
else
    run Rscript -e 'lints <- lintr::lint_dir(); print(lints); quit(status = length(lints) > 0)'
fi

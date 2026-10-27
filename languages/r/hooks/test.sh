#!/usr/bin/env bash
# Default test hook: testthat, on the package (loaded from source) when the
# directory is one, else on tests/testthat/ alone.
source "$(dirname "$0")/../../_hooks.sh"
[ -d tests/testthat ] || unknown "no tests/testthat/ directory"
if [ -f DESCRIPTION ]; then
    run Rscript -e 'testthat::test_local(stop_on_failure = TRUE)'
else
    run Rscript -e 'testthat::test_dir("tests/testthat", stop_on_failure = TRUE)'
fi

#!/usr/bin/env bash
# Default test hook: testthat, on the package (loaded from source) when the
# directory is one, else on tests/testthat/ alone. Without tests/testthat/
# here, each package below that has one (xgboost's R-package/, packages
# side by side under r/).
source "$(dirname "$0")/../../_hooks.sh"
if [ ! -d tests/testthat ]; then
    _packages=()
    while IFS= read -r _dir; do
        if [ -d "${_dir}/tests/testthat" ]; then _packages+=("${_dir}"); fi
    done < <(dirs_below_with DESCRIPTION)
    [ "${#_packages[@]}" -eq 0 ] || run_hook_in "${_packages[@]}"
    unknown "no tests/testthat/ directory here or in the packages below"
fi
if [ -f DESCRIPTION ]; then
    run Rscript -e 'testthat::test_local(stop_on_failure = TRUE)'
else
    run Rscript -e 'testthat::test_dir("tests/testthat", stop_on_failure = TRUE)'
fi

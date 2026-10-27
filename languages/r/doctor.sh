#!/usr/bin/env bash
# Check that R's testthat package is installed, and lintr where a component
# configures it (.lintr), and Air where one has an air.toml.
source "$(dirname "$0")/../_common.sh"

_r_has() { Rscript -e "quit(status = !requireNamespace(\"$1\", quietly = TRUE))" >/dev/null 2>&1; }

if has Rscript; then
    _r_has testthat || problem "R package testthat is not installed"
    if impl_has r -name .lintr && ! _r_has lintr; then
        problem "R package lintr is not installed (a component has a .lintr)"
    fi
fi
if air_missing; then problem "Air ${_AIR_VERSION} not installed (a component has an air.toml)"; fi
doctor_done

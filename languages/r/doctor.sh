#!/usr/bin/env bash
# Check that R's testthat package is installed.
source "$(dirname "$0")/../_common.sh"

if has Rscript && ! Rscript -e 'quit(status = !requireNamespace("testthat", quietly = TRUE))' >/dev/null 2>&1; then
    problem "R package testthat is not installed"
fi
doctor_done

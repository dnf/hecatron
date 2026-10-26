# Smoke test for r: build and run a hello world in a scratch directory.
# shellcheck shell=bash
set -euo pipefail
Rscript -e 'library(testthat); test_that("hello", expect_equal(paste("hel", "lo", sep = ""), "hello")); cat("hello\n")'

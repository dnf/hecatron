#!/usr/bin/env bash
# Install R
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install r; brew_ensure_linked r Rscript; }
install_linux() { apt_install r-base r-cran-testthat; }

deps() {
    Rscript -e 'if (!requireNamespace("testthat", quietly = TRUE)) install.packages("testthat", repos = "https://cloud.r-project.org")'
}

log "Installing R..."
"install_${OS}"
deps

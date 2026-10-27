#!/usr/bin/env bash
# Install R
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install r; brew_ensure_linked r Rscript; }
install_linux() { apt_install r-base r-cran-testthat; }

deps() {
    _r_package testthat r-cran-testthat
    # The default lint hook runs lintr where a component configures it.
    if impl_has r -name .lintr; then
        _r_package lintr r-cran-lintr
    fi
}
# An R package from apt on Linux, else from CRAN.
_r_package() {
    _r_has "$1" && return
    if [ "${OS}" = "linux" ]; then
        apt_install "$2"
    else
        Rscript -e "install.packages(\"$1\", repos = \"https://cloud.r-project.org\")"
    fi
}
_r_has() { Rscript -e "quit(status = !requireNamespace(\"$1\", quietly = TRUE))" >/dev/null 2>&1; }

log "Installing R..."
"install_${OS}"
deps
# The dependency auditor hecatron audit uses for this language.
osv_scanner_install

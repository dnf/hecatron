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
    if air_missing; then _install_air; fi
    impl_deps r
}
# Air (hecatron format's R formatter), its release binary.
_install_air() {
    local _platform _sha256
    case "${OS}-$(uname -m)" in
        linux-x86_64) _platform=x86_64-unknown-linux-gnu _sha256=d2b9452bfc44d55f0e61c2d8906ca3fbaac5e63f40726017acf8bad6bfd3ba37 ;;
        linux-aarch64 | linux-arm64) _platform=aarch64-unknown-linux-gnu _sha256=b3d9d576dd0e2dae4be7a8c4fa78de6b2e23bc0ce703db84776c08fc17bcd55c ;;
        macos-x86_64) _platform=x86_64-apple-darwin _sha256=582221cd54220a4232c8251760a05d8765b56361bada9608a9d3150972ba101e ;;
        macos-arm64) _platform=aarch64-apple-darwin _sha256=30ca7b7458cc78a11d1de9cfe1a1154b805bd1c64fbf71f53b739f0b261b857e ;;
        *) echo "WARNING: no Air release for $(uname -sm)" >&2; return 0 ;;
    esac
    log "Installing Air ${_AIR_VERSION} (for hecatron format)..."
    release_binary_install air \
        "https://github.com/posit-dev/air/releases/download/${_AIR_VERSION}/air-${_platform}.tar.gz" "${_sha256}" "air-${_platform}/air"
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

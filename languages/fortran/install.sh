#!/usr/bin/env bash
# Install Fortran toolchain (gfortran + cmake)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install gcc cmake; }
install_linux() { apt_install gfortran cmake; }

# fpm, the Fortran Package Manager, where a component builds with an
# fpm.toml. It is built from the single-file source each release publishes,
# with the gfortran installed above: the release binaries cover x86_64 Linux
# only, and are linked against a particular GCC's runtime.
FPM_VERSION="0.13.0"
fpm_missing() {
    local _dir
    has fpm && return 1
    while IFS= read -r _dir; do
        [ -f "${_dir}/fpm.toml" ] && return 0
    done < <(impl_dirs fortran)
    return 1
}
_install_fpm() {
    local _tmp _bin_dir="/usr/local/bin" _sudo="${SUDO}"
    if [ "${OS}" = "macos" ]; then _bin_dir="$(brew --prefix)/bin"; _sudo=""; fi
    _tmp="$(mktemp -d)"
    log "Building fpm ${FPM_VERSION}..."
    curl -fsSL -o "${_tmp}/fpm.F90" \
        "https://github.com/fortran-lang/fpm/releases/download/v${FPM_VERSION}/fpm-${FPM_VERSION}.F90"
    (cd "${_tmp}" && gfortran -O2 -o fpm fpm.F90)
    ${_sudo} install -m 0755 "${_tmp}/fpm" "${_bin_dir}/fpm"
    rm -rf "${_tmp}"
}

deps() { if fpm_missing; then _install_fpm; fi; }

log "Installing Fortran..."
"install_${OS}"
deps

#!/usr/bin/env bash
# Install Rust via rustup
source "$(dirname "$0")/../_common.sh"

# rustup with the default version as its default toolchain. A rustup whose
# install was cut short (killed mid-download) leaves rustc as a proxy with no
# toolchain behind it, so check that rustc runs, and repair by installing the
# default toolchain through the rustup that's there.
_rustc_works() { rustc --version >/dev/null 2>&1; }
_install() {
    # shellcheck source=/dev/null
    [ -f "${HOME}/.cargo/env" ] && source "${HOME}/.cargo/env"
    _rustc_works && return
    if has rustup; then
        log "Repairing rustup: installing Rust $(default_version) as its default..."
        rustup toolchain install "$(default_version)" --profile default </dev/null
        rustup default "$(default_version)" </dev/null
    else
        curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs |
            sh -s -- -y --default-toolchain "$(default_version)"
        # shellcheck source=/dev/null
        source "${HOME}/.cargo/env"
    fi
    _rustc_works
}

install_macos() { _install; }
# rustc links through the system C compiler, which a fresh Linux box lacks.
install_linux() {
    has cc || apt_install build-essential
    _install
}

# Every other version the project's components need, side by side as rustup
# toolchains. rustup takes "1.80" or "1.80.1" itself; a shorter prefix ("1")
# is resolved to the newest release line it matches.
_toolchain() {
    local version
    version="$(bash "$(dirname "$0")/releases.sh" | cut -f1 | newest_match "$1")"
    echo "${version:-${1#=}}"
}
_install_versions() {
    local specs spec toolchain
    specs="$(missing_versions rust)"
    while IFS= read -r spec; do
        [ -n "${spec}" ] || continue
        toolchain="$(_toolchain "${spec}")"
        log "Installing Rust ${toolchain} with rustup..."
        rustup toolchain install "${toolchain}" --profile default </dev/null
    done <<<"${specs}"
}
# Components rustup leaves out of the default profile that crates need:
# rust-src for ui_test's UI tests (it builds them against the standard
# library's sources), in every toolchain the project uses.
_install_components() {
    local toolchain
    grep -qx ui_test <<<"$(cargo_lock_crates rust)" || return 0
    log "Adding rust-src, which ui_test's UI tests need..."
    while IFS= read -r toolchain; do
        [ -n "${toolchain}" ] || continue
        rustup component add rust-src --toolchain "${toolchain}" </dev/null
    done < <(rustup toolchain list | sed 's/ .*//')
}
deps() {
    _install_versions
    cargo_dependencies rust
    _install_components
}

log "Installing Rust..."
"install_${OS}"
deps
# The dependency auditor hecatron audit uses for this language.
osv_scanner_install

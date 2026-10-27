#!/usr/bin/env bash
# Install Rust via rustup
source "$(dirname "$0")/../_common.sh"

# rustup with the default version as its default toolchain.
_install() {
    if ! has rustc; then
        curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs |
            sh -s -- -y --default-toolchain "$(default_version)"
        # shellcheck source=/dev/null
        source "$HOME/.cargo/env"
    fi
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
deps() { _install_versions; }

log "Installing Rust..."
"install_${OS}"
deps

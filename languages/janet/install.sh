#!/usr/bin/env bash
# Install Janet
source "$(dirname "$0")/../_common.sh"

install_macos() {
    brew_install janet
}
install_linux() {
    # Janet is not in standard apt repos; build from source, with the C
    # toolchain a fresh machine lacks.
    { has make && has cc; } || apt_install build-essential
    local ver="1.37.1"
    local tmp
    tmp="$(mktemp -d)"
    cd "${tmp}" || return 1
    curl -sL "https://github.com/janet-lang/janet/archive/refs/tags/v${ver}.tar.gz" | tar xz
    cd "janet-${ver}" || return 1
    make -j"$(nproc)"
    ${SUDO} make install
    rm -rf "${tmp}"
}

deps() {
    :
}

log "Installing Janet..."
"install_${OS}"
deps

#!/usr/bin/env bash
# Install Janet, and jpm where a component has a project.janet
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
    curl -sL "https://github.com/janet-lang/janet/archive/refs/tags/v${ver}.tar.gz" | tar -C "${tmp}" -xz
    make -C "${tmp}/janet-${ver}" -j"$(nproc)"
    ${SUDO} make -C "${tmp}/janet-${ver}" install
    rm -rf "${tmp}"
}

# jpm, Janet's project tool, for the default hooks where a component has a
# project.janet (Homebrew's janet already includes it).
_JPM_VERSION="v1.2.0"
deps() {
    impl_has janet -name project.janet || return 0
    has jpm && return
    has git || apt_install git
    local tmp
    tmp="$(mktemp -d)"
    log "Installing jpm ${_JPM_VERSION}..."
    git clone --quiet --depth 1 --branch "${_JPM_VERSION}" https://github.com/janet-lang/jpm.git "${tmp}/jpm"
    (cd "${tmp}/jpm" && ${SUDO} janet bootstrap.janet)
    rm -rf "${tmp}"
}

log "Installing Janet..."
"install_${OS}"
deps

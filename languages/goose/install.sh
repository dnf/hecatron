#!/usr/bin/env bash
# Install Goose (https://aardappel.github.io/goose/) from its GitHub release
source "$(dirname "$0")/../_common.sh"

GOOSE_VERSION="v2026.3"

# Other tools are also called goose (a database migrator, an AI agent);
# only the language prints "goose runtime" from `goose version`.
has_goose_language() { goose version 2>/dev/null | grep -q '^goose runtime'; }

install_release() {
    local platform="$1"
    local archive="goose-${GOOSE_VERSION}-${platform}.tar.gz"
    curl -fsSL "https://github.com/aardappel/goose/releases/download/${GOOSE_VERSION}/${archive}" -o "/tmp/${archive}"
    ${SUDO} rm -rf /opt/goose
    ${SUDO} mkdir -p /opt/goose
    ${SUDO} tar -xzf "/tmp/${archive}" -C /opt/goose --strip-components=1
    rm "/tmp/${archive}"
    ${SUDO} ln -sf /opt/goose/goose /usr/local/bin/goose
}

install_macos() {
    if has_goose_language; then return; fi
    if [ "$(uname -m)" = "arm64" ]; then install_release macos-arm64; else install_release macos-x64; fi
}

install_linux() {
    if has_goose_language; then return; fi
    if [ "$(uname -m)" != "x86_64" ]; then
        log "Goose has no Linux $(uname -m) release; skipping"
        return
    fi
    install_release linux-x64
}

# A C compiler and libc's headers: goose runs a program by compiling its C
# in-process (TinyCC, which needs the headers) and builds with the
# machine's compiler.
deps() {
    [ "${OS}" = "linux" ] || return 0
    { has cc && [ -f /usr/include/stdint.h ]; } || apt_install build-essential
}

log "Installing Goose..."
"install_${OS}"
deps

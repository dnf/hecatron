#!/usr/bin/env bash
# Install the Pharo Smalltalk VM into ~/.pharo, with `pharo` on PATH
source "$(dirname "$0")/../_common.sh"

PHARO_VERSION="120"
PHARO_HOME="${HOME}/.pharo"
# ~/.local/bin rather than /usr/local/bin, which Apple Silicon Macs don't have
# and which needs root.
BIN_DIR="${HOME}/.local/bin"

# macOS: Pharo's zeroconf script; the launcher follows the symlink back to its
# VM.
install_macos() {
    has pharo && return
    mkdir -p "${PHARO_HOME}" "${BIN_DIR}"
    (cd "${PHARO_HOME}" && curl -fsSL "https://get.pharo.org/64/vm${PHARO_VERSION}" | bash)
    ln -sf "${PHARO_HOME}/pharo" "${BIN_DIR}/pharo"
}

# Linux: the zeroconf script only knows x86 Linux, so download this Pharo
# version's VM for the machine's architecture ourselves. The VM's launcher
# finds its libraries from the path it was run by, so `pharo` is a wrapper that
# runs it from where it is, headless as zeroconf's script runs it.
install_linux() {
    has unzip || apt_install unzip
    has pharo && return
    local zip
    zip="$(mktemp)"
    curl -fsSL -o "${zip}" "$(_pharo_vm_url)"
    rm -rf "${PHARO_HOME}/vm"
    mkdir -p "${PHARO_HOME}/vm" "${BIN_DIR}"
    unzip -q -o "${zip}" -d "${PHARO_HOME}/vm"
    rm -f "${zip}"
    printf '#!/bin/sh\nexec "%s/vm/bin/pharo" --headless "$@"\n' "${PHARO_HOME}" > "${BIN_DIR}/pharo"
    chmod +x "${BIN_DIR}/pharo"
}
# The stable VM, else the latest: Pharo publishes only a latest VM for some
# architectures (aarch64).
_pharo_vm_url() {
    local base status
    base="https://files.pharo.org/get-files/${PHARO_VERSION}/pharo-vm-Linux-$(uname -m)"
    for status in stable latest; do
        if curl -fsSI -o /dev/null "${base}-${status}.zip"; then
            echo "${base}-${status}.zip"
            return
        fi
    done
    echo "error: no Pharo ${PHARO_VERSION} VM for Linux $(uname -m)" >&2
    return 1
}

# A project's test hook downloads the Pharo image it tests against.
deps() { :; }

log "Installing Pharo..."
"install_${OS}"
deps

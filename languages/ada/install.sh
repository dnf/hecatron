#!/usr/bin/env bash
# Install Ada toolchain (GNAT + GPRbuild via Alire on macOS, apt on Linux)
source "$(dirname "$0")/../_common.sh"

ALIRE_VERSION="2.1.1"

_alire_toolchain_bin() {
    local pattern="$1"
    local dir
    for dir in "$HOME/.local/share/alire/toolchains/${pattern}"*/bin; do
        if [ -d "$dir" ]; then
            echo "$dir"
            return
        fi
    done
}

_add_alire_to_path() {
    local gnat_bin gprbuild_bin
    gnat_bin=$(_alire_toolchain_bin "gnat_native")
    gprbuild_bin=$(_alire_toolchain_bin "gprbuild")
    [ -n "$gnat_bin" ] && export PATH="$gnat_bin:$PATH"
    [ -n "$gprbuild_bin" ] && export PATH="$gprbuild_bin:$PATH"
}

_install_alire() {
    if has alr; then return; fi
    local arch; arch="$(uname -m)"
    if [ "$arch" = "arm64" ]; then arch="aarch64"; fi
    local os_name="${OS}"
    local url="https://github.com/alire-project/alire/releases/download/v${ALIRE_VERSION}/alr-${ALIRE_VERSION}-bin-${arch}-${os_name}.zip"
    local tmp; tmp="$(mktemp -d)"
    log "  Downloading Alire ${ALIRE_VERSION}..."
    curl -fsSL "$url" -o "$tmp/alr.zip"
    unzip -q "$tmp/alr.zip" -d "$tmp"
    mkdir -p "$HOME/.local/bin"
    mv "$tmp/bin/alr" "$HOME/.local/bin/"
    rm -rf "$tmp"
    export PATH="$HOME/.local/bin:$PATH"
}

install_macos() {
    has unzip || apt_install unzip
    _install_alire
    if ! has gnat || ! has gprbuild; then
        log "  Installing GNAT + GPRbuild via Alire..."
        alr toolchain --select gnat_native gprbuild
        _add_alire_to_path
    fi
    if has gnat; then
        log "  gnat: $(gnat --version 2>&1 | head -1)"
    fi
    echo ""
    echo "Add the following to your shell profile to use gnat/gprbuild:"
    echo ""
    for dir in "$HOME/.local/share/alire/toolchains"/*/bin; do
        [ -d "$dir" ] && echo "  export PATH=\"$dir:\$PATH\""
    done
    echo ""
}

install_linux() { apt_install gnat gprbuild; }

# Alire where a component is a crate (alire.toml), using the GNAT and
# GPRbuild installed above rather than downloading its own.
alire_needed() {
    local _dir
    while IFS= read -r _dir; do
        [ -f "${_dir}/alire.toml" ] && return 0
    done < <(impl_dirs ada)
    return 1
}
_alire_use_system_toolchain() {
    alr settings --global --get toolchain.use.gnat >/dev/null 2>&1 && return
    log "  Selecting the system GNAT and GPRbuild for Alire..."
    alr --non-interactive toolchain --select gnat_external gprbuild
}

deps() {
    [ "${OS}" = "linux" ] && alire_needed || return 0
    has unzip || apt_install unzip
    _install_alire
    _alire_use_system_toolchain
}

log "Installing Ada..."
"install_${OS}"
deps

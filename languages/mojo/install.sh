#!/usr/bin/env bash
# Install Mojo (the PyPI package, as a uv tool on PATH in ~/.local/bin)
source "$(dirname "$0")/../_common.sh"

# Keep in step with the pixi pin in workflow.yml: Mojo changes the language
# between releases, so the code builds on this version only.
MOJO_VERSION="0.26.1.0"

# The `mojo` command comes from the mojo-compiler dependency, so its
# executables are installed too.
_install() {
    if grep -qF "Mojo ${MOJO_VERSION} " <<<"$(mojo --version 2>/dev/null)"; then
        return
    fi
    if ! has uv; then
        bash "$(dirname "$0")/../python/install.sh"
        # uv lands in ~/.local/bin, which this shell's PATH may predate.
        export PATH="${HOME}/.local/bin:${PATH}"
    fi
    uv tool install --force "mojo==${MOJO_VERSION}" --with-executables-from mojo-compiler
}

install_macos() {
    if [ "$(uname -m)" = "x86_64" ]; then
        echo "Mojo has no build for Intel Macs (only Apple silicon); skipping." >&2
        return 1
    fi
    _install
}
install_linux() { _install; }

deps() { :; }

log "Installing Mojo ${MOJO_VERSION}..."
"install_${OS}"
deps

#!/usr/bin/env bash
# Install Python 3 + uv
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install python uv; }
install_linux() {
    apt_install python3 python3-venv
    if ! has uv; then
        curl -LsSf https://astral.sh/uv/install.sh | sh
    fi
}

deps() { (cd "$ROOT/src/python" && uv sync --extra dev); }

log "Installing Python..."
"install_${OS}"
deps

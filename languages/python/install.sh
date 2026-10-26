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

# A .venv copied or moved from another checkout keeps scripts whose shebangs
# point at the old location, so `uv run pyright` can't start; rebuild it.
_sync_venv() {
    if [ -f .venv/bin/pytest ] && ! head -1 .venv/bin/pytest | grep -qF "${PWD}/.venv/"; then
        log "Recreating moved virtualenv in ${PWD}"
        rm -rf .venv
    fi
    uv sync --extra dev
}
deps() { each_impl python _sync_venv; }

log "Installing Python..."
"install_${OS}"
deps

#!/usr/bin/env bash
# Install Python 3 + uv
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install python uv; }
install_linux() {
    apt_install python3 python3-venv
    if ! has uv; then
        curl -LsSf https://astral.sh/uv/install.sh | sh
    fi
    # uv lands in ~/.local/bin, which this shell's PATH may predate.
    export PATH="${HOME}/.local/bin:${PATH}"
}

# A .venv copied or moved from another checkout keeps scripts whose shebangs
# point at the old location, so `uv run pyright` can't start; rebuild it.
_sync_venv() {
    [ -f pyproject.toml ] || return 0
    if [ -f .venv/bin/pytest ] && ! head -1 .venv/bin/pytest | grep -qF "${PWD}/.venv/"; then
        log "Recreating moved virtualenv in ${PWD}"
        rm -rf .venv
    fi
    uv sync --extra dev
}
# Every Python version the project's components resolve to (pin files, the
# project's config, else the default; docs/design.md §18), through uv, which
# takes a prefix ("3.12") or an exact version.
_install_versions() {
    [ -n "${HECATRON_HOME:-}" ] || return 0
    local spec
    while IFS=$'\t' read -r spec _; do
        [ "${spec}" = "-" ] && continue
        log "Installing Python ${spec#=} with uv..."
        uv python install "${spec#=}" </dev/null
    done < <(cd "${ROOT}" && python3 "${HECATRON_HOME}/libexec/lib/project.py" versions python)
}
deps() {
    _install_versions
    each_impl python _sync_venv
}

log "Installing Python..."
"install_${OS}"
deps

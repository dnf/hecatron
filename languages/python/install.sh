#!/usr/bin/env bash
# Install Python 3 + uv
source "$(dirname "$0")/../_common.sh"

# uv from Astral's installer on both platforms, into ~/.local/bin: Homebrew's
# uv trails Astral's releases, and a uv's list of Python builds is fixed at
# its release, so an older one can't install a newer Python. The installer's
# uv can update itself (_uv_python_install).
install_macos() {
    brew_install python
    uv_install
}
# A dependency with no wheel for this platform or Python version builds from
# its source distribution, which for a C extension needs a C compiler (the
# Pythons uv installs bring their own headers, and maturin fetches Rust for a
# Rust extension itself).
install_linux() {
    apt_install python3 python3-venv
    has cc || apt_install build-essential
    uv_install
}
# A Python released after this uv has no build in its list: update uv, then
# try again.
_uv_python_install() {
    uv python install "$1" </dev/null && return
    log "Updating uv, which has no build of Python $1..."
    uv self update </dev/null
    uv python install "$1" </dev/null
}

# A .venv copied or moved from another checkout keeps scripts whose shebangs
# point at the old location, so `uv run pyright` can't start; rebuild it.
_sync_venv() {
    [ -f pyproject.toml ] || return 0
    if [ -f .venv/bin/pytest ] && ! head -1 .venv/bin/pytest | grep -qF "${PWD}/.venv/"; then
        log "Recreating moved virtualenv in ${PWD}"
        rm -rf .venv
    fi
    uv sync --all-extras
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
        _uv_python_install "${spec#=}"
    done < <(cd "${ROOT}" && python3 "${HECATRON_HOME}/libexec/lib/project.py" versions python)
}
deps() {
    _install_versions
    each_impl python _sync_venv
}

log "Installing Python..."
"install_${OS}"
deps

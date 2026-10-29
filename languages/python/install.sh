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

# Every Python version the project's components resolve to (pin files, the
# project's config, else the default; docs/guide/versions.md), through uv, which
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
    # Each component's environment (a .venv moved from another checkout,
    # whose scripts can't start, is made again: deps_python).
    impl_deps python
}

log "Installing Python..."
"install_${OS}"
bash "$(dirname "$0")/link-python.sh"
deps
# OSV-Scanner, which hecatron licenses reads this language's licenses with.
osv_scanner_install

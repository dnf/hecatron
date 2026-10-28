#!/usr/bin/env bash
# Uninstall Python: the Python builds install.sh installed with uv, and
# Homebrew's python. The system's python3 stays (the operating system, and
# hecatron itself, run on it), and so does uv, which the C, C++, Objective-C
# and Mojo installers also install.
source "$(dirname "$0")/../_common.sh"

_uv() {
    if has uv; then uv "$@"; else "${HOME}/.local/bin/uv" "$@"; fi
}
_has_uv() { has uv || [ -x "${HOME}/.local/bin/uv" ]; }

# Mojo is a uv tool, whose environment runs on one of uv's Python builds.
_remove_uv_pythons() {
    _has_uv || return 0
    if grep -q '^mojo ' <<<"$(_uv tool list 2>/dev/null)"; then
        left_shared "uv's Python builds" "Mojo runs on one."
    else
        _uv python uninstall --all 2>/dev/null || true
    fi
    left_shared uv "C, C++, Objective-C and Mojo use it too."
}

uninstall_macos() {
    _remove_uv_pythons
    brew uninstall python 2>/dev/null || true
}
uninstall_linux() {
    _remove_uv_pythons
    if has python3; then
        left_shared "the system's python3" "the operating system depends on it."
    fi
}

log "Uninstalling Python..."
"uninstall_${OS}"
# install.sh's `python` link to python3, where there was no python.
rm -rf "${HOME}/.local/share/hecatron/python"

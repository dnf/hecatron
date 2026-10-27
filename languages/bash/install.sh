#!/usr/bin/env bash
# Install Bash, ShellCheck and (where a component has tests for it) Bats
source "$(dirname "$0")/../_common.sh"

install_macos() {
    # macOS ships with Bash 3.2; install modern Bash via Homebrew
    brew_install bash
}
install_linux() {
    apt_install bash
}

# The default hooks lint with ShellCheck, and test with Bats where a
# component has .bats files.
deps() {
    has shellcheck || _install_tool shellcheck shellcheck
    if impl_has bash -name '*.bats' && ! has bats; then
        _install_tool bats-core bats
    fi
}
_install_tool() {
    if [ "${OS}" = "macos" ]; then brew_install "$1"; else apt_install "$2"; fi
}

log "Installing Bash..."
"install_${OS}"
deps

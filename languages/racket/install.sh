#!/usr/bin/env bash
# Install Racket
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install minimal-racket; }
install_linux() { apt_install racket; }

# minimal-racket (Homebrew) has no rackunit and no `raco test` (compiler-lib);
# the full distribution (apt, CI's setup-racket) already includes both.
# (Only when missing: on the full distribution they are installed in
# installation scope, and installing them again in user scope fails.)
deps() {
    raco test --help >/dev/null 2>&1 && racket -l rackunit -e '' >/dev/null 2>&1 && return
    raco pkg install --auto --skip-installed rackunit-lib compiler-lib
}

log "Installing Racket..."
"install_${OS}"
deps

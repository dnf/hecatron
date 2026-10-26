#!/usr/bin/env bash
# Install Racket
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install minimal-racket; }
install_linux() { apt_install racket; }

# minimal-racket (Homebrew) has no rackunit and no `raco test` (compiler-lib);
# the full distribution (apt, CI's setup-racket) already includes both.
deps() { raco pkg install --auto --skip-installed rackunit-lib compiler-lib; }

log "Installing Racket..."
"install_${OS}"
deps

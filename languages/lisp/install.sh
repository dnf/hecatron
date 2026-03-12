#!/usr/bin/env bash
# Install SBCL (Steel Bank Common Lisp)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install sbcl; }
install_linux() { apt_install sbcl; }

deps() { :; }

log "Installing SBCL..."
"install_${OS}"
deps

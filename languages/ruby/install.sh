#!/usr/bin/env bash
# Install Ruby
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install ruby; }
install_linux() { apt_install ruby ruby-bundler; }

deps() { (cd "$ROOT/src/ruby" && bundle install); }

log "Installing Ruby..."
"install_${OS}"
deps

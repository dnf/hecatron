#!/usr/bin/env bash
# Install Elixir (pulls in Erlang/OTP as a dependency)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install elixir; }
install_linux() { apt_install elixir; }

deps() { :; }

log "Installing Elixir..."
"install_${OS}"
deps

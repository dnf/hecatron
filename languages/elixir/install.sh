#!/usr/bin/env bash
# Install Elixir (pulls in Erlang/OTP as a dependency)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install elixir; }
install_linux() { apt_install elixir; }

# Hex and rebar3, which mix needs to fetch and build dependencies; a fresh
# Elixir has neither, and mix asks interactively.
deps() {
    impl_has elixir -name mix.exs || return 0
    mix local.hex --force --if-missing
    mix local.rebar --force --if-missing
}

log "Installing Elixir..."
"install_${OS}"
deps
# The dependency auditor hecatron audit uses for this language.
osv_scanner_install

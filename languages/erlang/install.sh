#!/usr/bin/env bash
# Install Erlang/OTP + rebar3
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install erlang rebar3; }
install_linux() { apt_install erlang-dev erlang-tools rebar3; }

deps() { :; }

log "Installing Erlang..."
"install_${OS}"
deps

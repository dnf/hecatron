#!/usr/bin/env bash
# Uninstall Erlang/OTP + rebar3
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall erlang rebar3 2>/dev/null || true; }
uninstall_linux() { $SUDO apt-get remove -y erlang-dev erlang-tools rebar3 2>/dev/null || true; }

log "Uninstalling Erlang..."
"uninstall_${OS}"

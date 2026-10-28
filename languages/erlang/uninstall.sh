#!/usr/bin/env bash
# Uninstall rebar3. Erlang/OTP stays: Elixir and Gleam run on it, and their
# installers install it.
source "$(dirname "$0")/../_common.sh"

_leave_otp() {
    if has erl; then left_shared Erlang/OTP "Elixir and Gleam run on it."; fi
}

uninstall_macos() {
    brew uninstall rebar3 2>/dev/null || true
    _leave_otp
}
uninstall_linux() {
    $SUDO apt-get remove -y rebar3 2>/dev/null || true
    _leave_otp
}

log "Uninstalling Erlang..."
"uninstall_${OS}"

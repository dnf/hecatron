#!/usr/bin/env bash
# Uninstall what install.sh installed: hecatron's OTP builds and rebar3, and
# rebar3's package. A distribution's or Homebrew's Erlang/OTP stays: Elixir
# and Gleam run on it, and their installers install it.
source "$(dirname "$0")/../_common.sh"

_leave_otp() {
    if has erl; then left_shared Erlang/OTP "Elixir and Gleam run on it."; fi
}

uninstall_macos() {
    brew uninstall rebar3 2>/dev/null || true
    _leave_otp
}
uninstall_linux() {
    toolchain_uninstall erlang
    rm -f "$(hecatron_bin)/rebar3"
    $SUDO apt-get remove -y rebar3 2>/dev/null || true
    _leave_otp
}

log "Uninstalling Erlang..."
"uninstall_${OS}"

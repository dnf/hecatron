#!/usr/bin/env bash
# Uninstall Gleam: hecatron's downloads, and where older installers put it.
# Erlang/OTP stays: Erlang and Elixir use it too.
source "$(dirname "$0")/../_common.sh"

log "Uninstalling Gleam..."
toolchain_uninstall gleam
if [ "${OS}" = "macos" ]; then
    brew uninstall gleam 2>/dev/null || true
else
    $SUDO rm -f /usr/local/bin/gleam
fi

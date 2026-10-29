#!/usr/bin/env bash
# Install Erlang/OTP and rebar3. On Ubuntu, builds.hex.pm's OTP build at the
# version config.json pins, into hecatron's toolchains directory with its
# commands linked into hecatron's bin directory, and rebar3's release
# escript there too: Ubuntu's own Erlang is older than current libraries and
# Gleam's standard library need (24.04's is OTP 25; they need 26).
# builds.hex.pm has no builds for Debian, which gets its distribution's
# (Debian 13's is OTP 27) with erlang-dev's headers; macOS gets Homebrew's.
# Elixir's and Gleam's installers run this one.
source "$(dirname "$0")/../_common.sh"

# rebar3's release, pinned and checked against the SHA-256 GitHub records
# for it: an escript, which runs on the OTP on PATH.
_REBAR3_VERSION="3.27.3"
_REBAR3_SHA256="f1d87d78decfc56b87d971c75ee6355698ec0c49fadd57f2c056043b7ca291b5"
# The commands in an OTP build's bin/ that a shell runs.
_OTP_COMMANDS="ct_run dialyzer epmd erl erl_call erlc escript run_erl to_erl typer"

_rebar3_current() {
    [ "$("$(hecatron_bin)/rebar3" version 2>/dev/null </dev/null | awk '{print $2}')" = "${_REBAR3_VERSION}" ]
}
_install_otp() {
    local home command
    home="$(otp_install_ubuntu "=$(default_version)" "$(hecatron_data)/toolchains/erlang")"
    for command in ${_OTP_COMMANDS}; do
        if [ -x "${home}/bin/${command}" ]; then toolchain_link "${home}/bin" "${command}"; fi
    done
    _rebar3_current || release_binary_install rebar3 \
        "https://github.com/erlang/rebar3/releases/download/${_REBAR3_VERSION}/rebar3" "${_REBAR3_SHA256}"
}

install_macos() { brew_install erlang rebar3; }
install_linux() {
    if otp_builds_url >/dev/null; then
        _install_otp
    else
        apt_install erlang-dev erlang-tools rebar3
    fi
}

log "Installing Erlang..."
"install_${OS}"

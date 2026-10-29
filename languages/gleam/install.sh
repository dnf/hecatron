#!/usr/bin/env bash
# Install Gleam: the release for this machine at the version config.json
# pins, checked against its published SHA-256, into hecatron's toolchains
# directory (where the binary installs it from install.json), and Erlang/OTP,
# which Gleam's compiled code runs on (shared with Erlang and Elixir).
source "$(dirname "$0")/../_common.sh"

_version="$(default_version gleam)"
case "${OS}-$(uname -m)" in
    linux-x86_64) _target="x86_64-unknown-linux-musl" ;;
    linux-aarch64) _target="aarch64-unknown-linux-musl" ;;
    macos-x86_64) _target="x86_64-apple-darwin" ;;
    macos-arm64) _target="aarch64-apple-darwin" ;;
    *) echo "No Gleam release for $(uname -sm)" >&2; exit 1 ;;
esac
_home="$(toolchain_home gleam "${_version}")"

log "Installing Gleam ${_version}..."
if ! "${_home}/gleam" --version >/dev/null 2>&1; then
    _url="https://github.com/gleam-lang/gleam/releases/download/v${_version}/gleam-v${_version}-${_target}.tar.gz"
    toolchain_release_install gleam "${_version}" "${_url}" "$(curl -fsSL "${_url}.sha256" | cut -d' ' -f1)" 0
fi
toolchain_link "${_home}" gleam
# Gleam compiles to Erlang, and runs (and tests) on the BEAM: an OTP its
# standard library supports (26 or newer), else Erlang's installer's.
otp_at_least 26 || bash "$(dirname "$0")/../erlang/install.sh"

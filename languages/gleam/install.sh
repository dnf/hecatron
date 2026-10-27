#!/usr/bin/env bash
# Install Gleam (requires Erlang/OTP — shared with Erlang and Elixir)
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install gleam; }
install_linux() {
    local version="1.14.0"
    local arch
    arch="$(uname -m)"
    case "$arch" in
        x86_64)  arch="x86_64" ;;
        aarch64) arch="aarch64" ;;
        *)       echo "Unsupported architecture: $arch" >&2; exit 1 ;;
    esac
    local url="https://github.com/gleam-lang/gleam/releases/download/v${version}/gleam-v${version}-${arch}-unknown-linux-musl.tar.gz"
    curl -fsSL "$url" | $SUDO tar xz -C /usr/local/bin gleam
    # Gleam compiles to Erlang, and runs (and tests) on the BEAM.
    has erl || bash "$(dirname "$0")/../erlang/install.sh"
}

deps() { :; }

log "Installing Gleam..."
"install_${OS}"
deps

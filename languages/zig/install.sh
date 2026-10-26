#!/usr/bin/env bash
# Install Zig
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install zig; }
# The latest release, from ziglang.org's index of downloads (which also has
# each archive's URL: its naming changed after 0.14).
install_linux() {
    has zig && return
    local url
    url="$(curl -fsSL https://ziglang.org/download/index.json | python3 -c '
import json, sys
releases = json.load(sys.stdin)
latest = max((v for v in releases if v != "master"), key=lambda v: tuple(int(p) for p in v.split(".")))
print(releases[latest][sys.argv[1] + "-linux"]["tarball"])
' "$(uname -m)")"
    log "Downloading ${url##*/}..."
    $SUDO rm -rf /opt/zig
    $SUDO mkdir -p /opt/zig
    curl -fsSL "${url}" | $SUDO tar -C /opt/zig --strip-components=1 -xJ
    $SUDO ln -sf /opt/zig/zig /usr/local/bin/zig
}

deps() { :; }

log "Installing Zig..."
"install_${OS}"
deps

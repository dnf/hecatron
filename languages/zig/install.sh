#!/usr/bin/env bash
# Install Zig
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install zig; }
install_linux() {
    if ! has zig; then
        ARCH="$(dpkg --print-architecture)"
        ZIG_ARCH="$ARCH"
        if [ "$ARCH" = "amd64" ]; then ZIG_ARCH="x86_64"; fi
        if [ "$ARCH" = "arm64" ]; then ZIG_ARCH="aarch64"; fi
        $SUDO mkdir -p /opt/zig
        curl -fsSL "https://ziglang.org/download/0.14.0/zig-linux-${ZIG_ARCH}-0.14.0.tar.xz" | \
            $SUDO tar -C /opt/zig --strip-components=1 -xJ
        $SUDO ln -sf /opt/zig/zig /usr/local/bin/zig
    fi
}

deps() { :; }

log "Installing Zig..."
"install_${OS}"
deps

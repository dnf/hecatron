#!/usr/bin/env bash
# Install Swift
source "$(dirname "$0")/../_common.sh"

install_macos() { log "Using Xcode Swift (no install needed)."; }
install_linux() {
    if ! has swift; then
        ARCH="$(dpkg --print-architecture)"
        # Swift URL pattern differs by architecture:
        #   x86: ubuntu2404/swift-VER-RELEASE/swift-VER-RELEASE-ubuntu24.04.tar.gz
        #   arm: ubuntu2404-aarch64/swift-VER-RELEASE/swift-VER-RELEASE-ubuntu24.04-aarch64.tar.gz
        VER="5.10.1"
        if [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then
            SWIFT_URL="https://download.swift.org/swift-${VER}-release/ubuntu2404-aarch64/swift-${VER}-RELEASE/swift-${VER}-RELEASE-ubuntu24.04-aarch64.tar.gz"
        else
            SWIFT_URL="https://download.swift.org/swift-${VER}-release/ubuntu2404/swift-${VER}-RELEASE/swift-${VER}-RELEASE-ubuntu24.04.tar.gz"
        fi
        curl -fsSL "$SWIFT_URL" | $SUDO tar -C /usr/local --strip-components=2 -xz
    fi
}

deps() { :; }

log "Installing Swift..."
"install_${OS}"
deps

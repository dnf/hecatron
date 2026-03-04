#!/usr/bin/env bash
# Install Dart
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install dart-sdk; }
install_linux() {
    if ! has dart; then
        ARCH="$(dpkg --print-architecture)"
        curl -fsSL "https://storage.googleapis.com/dart-archive/channels/stable/release/latest/sdk/dartsdk-linux-${ARCH}-release.zip" -o /tmp/dart-sdk.zip
        $SUDO unzip -q -o /tmp/dart-sdk.zip -d /opt
        rm -f /tmp/dart-sdk.zip
        $SUDO ln -sf /opt/dart-sdk/bin/dart /usr/local/bin/dart
    fi
}

deps() {
    if [ -d "age/impl/dart" ] && has dart; then
        (cd src/dart && dart pub get)
    fi
}

log "Installing Dart..."
"install_${OS}"
deps

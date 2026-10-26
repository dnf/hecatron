#!/usr/bin/env bash
# Install Dart
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install dart-sdk; }
install_linux() {
    if ! has dart; then
        local arch zip
        arch="x64"
        [ "$(uname -m)" = "aarch64" ] && arch="arm64"
        zip="$(mktemp)"
        curl -fsSL "https://storage.googleapis.com/dart-archive/channels/stable/release/latest/sdk/dartsdk-linux-${arch}-release.zip" -o "${zip}"
        $SUDO rm -rf /opt/dart-sdk
        $SUDO unzip -q -o "${zip}" -d /opt
        rm -f "${zip}"
        $SUDO ln -sf /opt/dart-sdk/bin/dart /usr/local/bin/dart
    fi
}

# Only libraries with a pubspec.yaml have packages to get.
_pub_get() { [ ! -f pubspec.yaml ] || dart pub get; }
deps() { each_impl dart _pub_get; }

log "Installing Dart..."
"install_${OS}"
deps

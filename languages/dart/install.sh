#!/usr/bin/env bash
# Install Dart, and Flutter when a component is a Flutter package
source "$(dirname "$0")/../_common.sh"

# Flutter's own release archive, pinned and checked against the checksum
# Flutter publishes (storage.googleapis.com/flutter_infra_release/releases/
# releases_linux.json). Flutter publishes Linux archives for x64 only; on
# arm64 the same release comes from its git tag, which is how Flutter
# installs there.
_FLUTTER_VERSION="3.47.8"
_FLUTTER_LINUX_X64_SHA256="baf7dcc5a3dbba60fe863bf493a6cf203195621f7786eaa5a1bba02cd0d87dd2"
_FLUTTER_ROOT="/opt/flutter"

install_macos() {
    brew_install dart-sdk
    if flutter_missing; then brew install --cask flutter || true; fi
}
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
    if flutter_missing; then _install_flutter_linux; fi
}
# The flutter tool writes into its own tree (bin/cache, its lock file, the
# engine artifacts it fetches on first use), so the tree belongs to the user
# who installs it, as Flutter's install instructions have it.
_install_flutter_linux() {
    local archive
    has git || apt_install git
    has xz || apt_install xz-utils
    $SUDO rm -rf "${_FLUTTER_ROOT}"
    if [ "$(uname -m)" = "x86_64" ]; then
        archive="$(mktemp)"
        log "Downloading Flutter ${_FLUTTER_VERSION}..."
        curl -fsSL -o "${archive}" \
            "https://storage.googleapis.com/flutter_infra_release/releases/stable/linux/flutter_linux_${_FLUTTER_VERSION}-stable.tar.xz"
        if [ "$(sha256sum "${archive}" | cut -d' ' -f1)" != "${_FLUTTER_LINUX_X64_SHA256}" ]; then
            echo "Checksum mismatch for Flutter ${_FLUTTER_VERSION}" >&2
            rm -f "${archive}"
            return 1
        fi
        $SUDO mkdir -p "${_FLUTTER_ROOT}"
        $SUDO tar -C "${_FLUTTER_ROOT}" --strip-components=1 -xJf "${archive}"
        rm -f "${archive}"
    else
        log "Cloning Flutter ${_FLUTTER_VERSION}..."
        $SUDO git clone --quiet --depth 1 --branch "${_FLUTTER_VERSION}" \
            https://github.com/flutter/flutter.git "${_FLUTTER_ROOT}"
    fi
    $SUDO chown -R "$(id -u):$(id -g)" "${_FLUTTER_ROOT}"
    $SUDO ln -sf "${_FLUTTER_ROOT}/bin/flutter" /usr/local/bin/flutter
    # No analytics prompt or report from a toolchain hecatron set up.
    flutter --disable-analytics >/dev/null 2>&1 || true
}

# Each package's dependencies (hecatron deps: pub get, a Flutter package's
# through flutter).
deps() { impl_deps dart; }

log "Installing Dart..."
"install_${OS}"
deps
# The dependency auditor hecatron audit uses for this language.
osv_scanner_install

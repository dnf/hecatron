#!/usr/bin/env bash
# Install Odin
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install odin; }
install_linux() {
    if ! has odin; then
        ARCH="$(dpkg --print-architecture)"
        ODIN_ARCH="${ARCH}"
        if [ "${ARCH}" = "amd64" ]; then ODIN_ARCH="ubuntu_amd64"; fi
        if [ "${ARCH}" = "arm64" ]; then ODIN_ARCH="ubuntu_amd64"; fi
        ODIN_VERSION="dev-2025-03"
        $SUDO mkdir -p /opt/odin
        curl -fsSL "https://github.com/odin-lang/Odin/releases/download/${ODIN_VERSION}/odin-${ODIN_ARCH}-${ODIN_VERSION}.zip" -o /tmp/odin.zip
        $SUDO unzip -qo /tmp/odin.zip -d /opt/odin
        rm /tmp/odin.zip
        $SUDO ln -sf /opt/odin/odin /usr/local/bin/odin
    fi
}

deps() { :; }

log "Installing Odin..."
"install_${OS}"
deps

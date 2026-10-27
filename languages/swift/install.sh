#!/usr/bin/env bash
# Install Swift
source "$(dirname "$0")/../_common.sh"

install_macos() { log "Using Xcode Swift (no install needed)."; }

_swift_root="/opt/swift"

# swift.org's build for this distribution, latest release. It goes into its
# own prefix: merged into /usr/local, SwiftPM can't find swiftc ("Unable to
# discover swiftc"). Only the Swift tools are linked onto PATH; the toolchain
# also has clang and lldb, which would shadow the system's. The packages are
# the toolchain's documented runtime dependencies; a fresh machine lacks some
# (libncurses, the C toolchain the Swift driver links with), which leaves a
# swift that's on PATH but can't run, so they are installed even when swift is
# already there.
install_linux() {
    apt_install binutils g++ git libc6-dev libcurl4-openssl-dev libedit2 \
        libncurses-dev libpython3-dev libsqlite3-0 libxml2-dev libz3-dev \
        pkg-config tzdata unzip zlib1g-dev
    has swift || _download_swift
    local tool
    for tool in swift swiftc swift-format sourcekit-lsp; do
        if [ -x "${_swift_root}/usr/bin/${tool}" ]; then
            $SUDO ln -sfn "${_swift_root}/usr/bin/${tool}" "/usr/local/bin/${tool}"
        fi
    done
}

# The platform name swift.org publishes builds under, e.g. ubuntu24.04, debian13.
_swift_platform() {
    (. /etc/os-release && echo "${ID}${VERSION_ID}")
}
_latest_swift() {
    curl -fsSL https://www.swift.org/api/v1/install/releases.json |
        python3 -c 'import json, sys; print(json.load(sys.stdin)[-1]["name"])'
}
_download_swift() {
    local version platform suffix url
    version="$(_latest_swift)"
    platform="$(_swift_platform)"
    suffix=""
    [ "$(uname -m)" = "aarch64" ] && suffix="-aarch64"
    url="https://download.swift.org/swift-${version}-release/${platform//./}${suffix}/swift-${version}-RELEASE/swift-${version}-RELEASE-${platform}${suffix}.tar.gz"
    log "Downloading Swift ${version} for ${platform}${suffix}..."
    $SUDO rm -rf "${_swift_root}"
    $SUDO mkdir -p "${_swift_root}"
    if ! curl -fsSL "${url}" | $SUDO tar -C "${_swift_root}" --strip-components=1 -xz; then
        echo "error: no Swift ${version} build for ${platform}${suffix} (see https://www.swift.org/install/linux/)" >&2
        return 1
    fi
}

deps() { :; }

log "Installing Swift..."
"install_${OS}"
deps
# The dependency auditor hecatron audit uses for this language.
osv_scanner_install

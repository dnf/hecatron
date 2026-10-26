#!/usr/bin/env bash
# Install .NET SDK (C#) — shares .NET SDK with F#
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install dotnet; }
install_linux() {
    if ! dotnet --list-sdks 2>/dev/null | grep -q '^10\.'; then
        apt_has dotnet-sdk-10.0 || _add_microsoft_source
        apt_install dotnet-sdk-10.0
    fi
}

# Ubuntu packages .NET; Debian doesn't, so use Microsoft's repository there.
_add_microsoft_source() {
    local release deb
    release="$(. /etc/os-release && echo "${ID}/${VERSION_ID}")"
    deb="$(mktemp)"
    log "Adding Microsoft's apt repository for ${release}..."
    curl -fsSL -o "${deb}" "https://packages.microsoft.com/config/${release}/packages-microsoft-prod.deb"
    $SUDO dpkg -i "${deb}"
    rm -f "${deb}"
    $SUDO apt-get update -qq
}

deps() { :; }

log "Installing C#..."
"install_${OS}"
deps

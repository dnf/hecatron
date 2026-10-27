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

# The .NET runtimes the components' projects target, so their tests can run
# (dotnet_runtimes_missing in _common.sh): from apt where the distribution
# packages that version, else Microsoft's dotnet-install.sh, into the SDK's
# own directory.
_install_runtime() {
    local _root
    if [ "${OS}" = "linux" ] && apt_has "dotnet-runtime-$1.0"; then
        apt_install "dotnet-runtime-$1.0"
        return
    fi
    _root="$(dirname "$(dotnet --list-runtimes | sed -n 's/^Microsoft.NETCore.App [^ ]* \[\(.*\)\]$/\1/p' | head -1)")"
    _root="$(dirname "${_root}")"
    log "Installing the .NET $1 runtime into ${_root}..."
    curl -fsSL https://dot.net/v1/dotnet-install.sh |
        $SUDO bash -s -- --runtime dotnet --channel "$1.0" --install-dir "${_root}" --no-path
}
deps() {
    local _major
    while IFS= read -r _major; do
        [ -n "${_major}" ] && _install_runtime "${_major}"
    done < <(dotnet_runtimes_missing)
}

log "Installing C#..."
"install_${OS}"
deps

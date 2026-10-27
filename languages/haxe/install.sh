#!/usr/bin/env bash
# Install Haxe and haxelib
source "$(dirname "$0")/../_common.sh"

_haxe_root="/opt/haxe"

install_macos() {
    brew_install haxe
    _haxelib_setup
}

# The official release tarball, latest release, unpacked into its own
# prefix with haxe and haxelib linked into /usr/local/bin (haxe finds its
# standard library through the link). haxelib is a Neko program, so it needs
# the Neko VM and its standard library from apt.
install_linux() {
    apt_install neko
    _haxe_works || _download_haxe
    $SUDO ln -sfn "${_haxe_root}/haxe" /usr/local/bin/haxe
    $SUDO ln -sfn "${_haxe_root}/haxelib" /usr/local/bin/haxelib
    _haxelib_setup
}
_haxe_works() { [ -x "${_haxe_root}/haxe" ] && "${_haxe_root}/haxe" --version >/dev/null 2>&1; }
_haxe_platform() {
    case "$(uname -m)" in
        aarch64) echo "linux-arm64" ;;
        *)       echo "linux64" ;;
    esac
}
_download_haxe() {
    local latest version
    latest="$(curl -fsSLI -o /dev/null -w '%{url_effective}' https://github.com/HaxeFoundation/haxe/releases/latest)"
    version="${latest##*/}"
    log "Downloading Haxe ${version}..."
    $SUDO rm -rf "${_haxe_root}"
    $SUDO mkdir -p "${_haxe_root}"
    curl -fsSL "https://github.com/HaxeFoundation/haxe/releases/download/${version}/haxe-${version}-$(_haxe_platform).tar.gz" |
        $SUDO tar -C "${_haxe_root}" --strip-components=1 -xz
}

# haxelib refuses to run until it has a repository for libraries: the
# default one, in the home directory (no root needed to install into it).
_haxelib_setup() {
    haxelib config >/dev/null 2>&1 && return
    log "Setting up haxelib's repository in ~/haxelib..."
    mkdir -p "${HOME}/haxelib"
    haxelib setup "${HOME}/haxelib" </dev/null
}

log "Installing Haxe..."
"install_${OS}"

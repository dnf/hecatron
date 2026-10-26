#!/usr/bin/env bash
# Install Crystal
source "$(dirname "$0")/../_common.sh"

_crystal_root="/opt/crystal"

install_macos() { brew_install crystal; }

# The official build of the latest release for this architecture. Crystal's
# apt repository (crystal-lang.org/install.sh) has no arm64 packages. The
# packages are what the compiler links programs against.
install_linux() {
    apt_install build-essential pkg-config libpcre2-dev libevent-dev libssl-dev \
        libxml2-dev libyaml-dev libgmp-dev zlib1g-dev
    has crystal || _download_crystal
    local tool
    for tool in crystal shards; do
        $SUDO ln -sfn "${_crystal_root}/bin/${tool}" "/usr/local/bin/${tool}"
    done
}
_download_crystal() {
    local version arch
    version="$(curl -fsSLI -o /dev/null -w '%{url_effective}' https://github.com/crystal-lang/crystal/releases/latest)"
    version="${version##*/}"
    arch="$(uname -m)"
    log "Downloading Crystal ${version} for linux-${arch}..."
    $SUDO rm -rf "${_crystal_root}"
    $SUDO mkdir -p "${_crystal_root}"
    curl -fsSL "https://github.com/crystal-lang/crystal/releases/download/${version}/crystal-${version}-1-linux-${arch}.tar.gz" |
        $SUDO tar -C "${_crystal_root}" --strip-components=1 -xz
}

# Only libraries with a shard.yml have dependencies to install.
_shards_install() { [ ! -f shard.yml ] || shards install; }
deps() {
    if has crystal; then
        each_impl crystal _shards_install
    fi
}

log "Installing Crystal..."
"install_${OS}"
deps

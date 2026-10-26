#!/usr/bin/env bash
# Install Julia
source "$(dirname "$0")/../_common.sh"

_julia_root="/opt/julia"

install_macos() { brew_install julia; }

# The official tarball, latest stable release: Debian and Ubuntu don't package
# Julia. Unpacked into its own prefix and linked into /usr/local/bin.
install_linux() {
    has julia || _download_julia
    $SUDO ln -sfn "${_julia_root}/bin/julia" /usr/local/bin/julia
}
_julia_url() {
    curl -fsSL https://julialang-s3.julialang.org/bin/versions.json | python3 -c '
import json, sys
releases = json.load(sys.stdin)
def key(version):
    return tuple(int(part) for part in version.split("."))
latest = max((v for v, r in releases.items() if r["stable"] and v.replace(".", "").isdigit()), key=key)
print(next(f["url"] for f in releases[latest]["files"]
           if f["os"] == "linux" and f["arch"] == sys.argv[1] and f["kind"] == "archive" and f["extension"] == "tar.gz"))
' "$(uname -m)"
}
_download_julia() {
    local url
    url="$(_julia_url)"
    log "Downloading ${url##*/}..."
    $SUDO rm -rf "${_julia_root}"
    $SUDO mkdir -p "${_julia_root}"
    curl -fsSL "${url}" | $SUDO tar -C "${_julia_root}" --strip-components=1 -xz
}

deps() { :; }

log "Installing Julia..."
"install_${OS}"
deps

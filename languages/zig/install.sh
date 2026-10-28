#!/usr/bin/env bash
# Install Zig: the release for this machine at the version config.json
# pins, checked against the SHA-256 in ziglang.org's index of downloads, into
# hecatron's toolchains directory (where the binary installs it from
# install.json).
source "$(dirname "$0")/../_common.sh"

_version="$(default_version zig)"
case "${OS}-$(uname -m)" in
    linux-x86_64) _target="x86_64-linux" ;;
    linux-aarch64) _target="aarch64-linux" ;;
    macos-x86_64) _target="x86_64-macos" ;;
    macos-arm64) _target="aarch64-macos" ;;
    *) echo "No Zig release for $(uname -sm)" >&2; exit 1 ;;
esac
_home="$(toolchain_home zig "${_version}")"

log "Installing Zig ${_version}..."
if ! "${_home}/zig" version >/dev/null 2>&1; then
    _sha256="$(curl -fsSL https://ziglang.org/download/index.json | python3 -c '
import json, sys
print(json.load(sys.stdin)[sys.argv[1]][sys.argv[2]]["shasum"])
' "${_version}" "${_target}")"
    toolchain_release_install zig "${_version}" \
        "https://ziglang.org/download/${_version}/zig-${_target}-${_version}.tar.xz" "${_sha256}" 1
fi
toolchain_link "${_home}" zig

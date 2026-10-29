#!/usr/bin/env bash
# Install Zig: the release for this machine at the version config.json
# pins and every version the project's components need (a build.zig.zon's
# minimum_zig_version), side by side, each checked against the SHA-256 in
# ziglang.org's index of downloads, into hecatron's toolchains directory
# (where the binary installs them from install.json). The default's zig is
# linked into hecatron's bin directory.
source "$(dirname "$0")/../_common.sh"

case "${OS}-$(uname -m)" in
    linux-x86_64) _target="x86_64-linux" ;;
    linux-aarch64) _target="aarch64-linux" ;;
    macos-x86_64) _target="x86_64-macos" ;;
    macos-arm64) _target="aarch64-macos" ;;
    *) echo "No Zig release for $(uname -sm)" >&2; exit 1 ;;
esac
_index=""
_load_index() {
    [ -n "${_index}" ] || _index="$(curl -fsSL https://ziglang.org/download/index.json)"
}
# A field of the index's download of VERSION for this machine: its tarball
# (the index's, since Zig renamed its downloads in 0.14.1) or shasum.
_download_field() {
    _load_index
    python3 -c '
import json, sys
print(json.loads(sys.stdin.read())[sys.argv[1]][sys.argv[2]][sys.argv[3]])
' "$1" "${_target}" "$2" <<<"${_index}"
}
# The index's releases, one per line (not master, the nightly build).
_releases() {
    _load_index
    python3 -c 'import json, sys; print("\n".join(k for k in json.loads(sys.stdin.read()) if k != "master"))' <<<"${_index}"
}

_install_version() {
    local version="$1" home
    home="$(toolchain_home zig "${version}")"
    log "Installing Zig ${version}..."
    "${home}/zig" version >/dev/null 2>&1 && return
    toolchain_release_install zig "${version}" \
        "$(_download_field "${version}" tarball)" "$(_download_field "${version}" shasum)" 1
}

_default="$(default_version zig)"
_install_version "${_default}"
toolchain_link "$(toolchain_home zig "${_default}")" zig

# Every other version the components resolve to: a line (0.14) as its
# newest release.
while IFS= read -r _spec; do
    [ -n "${_spec}" ] || continue
    _version="$(_releases | newest_match "${_spec}")"
    if [ -z "${_version}" ]; then
        echo "No Zig release matches ${_spec}" >&2
        exit 1
    fi
    _install_version "${_version}"
done <<<"$(missing_versions zig)"

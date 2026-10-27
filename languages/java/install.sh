#!/usr/bin/env bash
# Install a JDK (shared by Java, Kotlin, Scala, Groovy): Eclipse Temurin, the
# default version and every version the project's components need, side by
# side.
source "$(dirname "$0")/../_common.sh"

# Where JDKs go: the directories the platform's own tools look in
# (/usr/libexec/java_home on macOS, update-java-alternatives and installed.sh
# on Linux). macOS's per-user one needs no sudo.
if [ "${OS}" = "macos" ]; then
    _jvm_root="${HOME}/Library/Java/JavaVirtualMachines"
    _sudo=""
    _api_os="mac"
else
    _jvm_root="/usr/lib/jvm"
    _sudo="${SUDO}"
    _api_os="linux"
fi
case "$(uname -m)" in
    x86_64) _api_arch="x64" ;;
    *) _api_arch="aarch64" ;;
esac

# macOS always has a /usr/bin/java stub; it works once java_home finds a JDK.
install_macos() {
    /usr/libexec/java_home >/dev/null 2>&1 || _install_jdk "=$(default_version)"
}
# The default JDK, linked into /usr/local/bin, which every shell has on PATH,
# when there is no java at all.
install_linux() {
    has java && return
    _install_jdk "=$(default_version)"
    local tool
    for tool in "${_jvm_root}/temurin-$(default_version)"/bin/*; do
        $SUDO ln -sfn "${tool}" "/usr/local/bin/$(basename "${tool}")"
    done
}

# Adoptium's general-availability releases of a feature version (the spec's
# first component), as "version<TAB>link<TAB>sha256", version without the
# build ("25.0.5" for jdk-25.0.5+8). The API pages 20 at a time, newest
# first, and answers 404 past the last page; an old checkout's JDK is on a
# later page.
_releases() {
    local feature="${1#=}" page=0 body
    while body="$(curl -fsSL "https://api.adoptium.net/v3/assets/feature_releases/${feature%%.*}/ga?os=${_api_os}&architecture=${_api_arch}&image_type=jdk&jvm_impl=hotspot&page_size=20&page=${page}" 2>/dev/null)"; do
        _release_lines <<<"${body}"
        [ "$(python3 -c 'import json, sys; print(len(json.load(sys.stdin)))' <<<"${body}")" -lt 20 ] && break
        page=$((page + 1))
    done
}
_release_lines() {
    python3 -c '
import json, sys
for release in json.load(sys.stdin):
    package = release["binaries"][0]["package"]
    version = release["release_name"].removeprefix("jdk-").split("+")[0]
    print(version + "\t" + package["link"] + "\t" + package["checksum"])
'
}
_sha256() { if has sha256sum; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1; }
_install_jdk() {
    local releases version link checksum archive home
    releases="$(_releases "$1")"
    version="$(cut -f1 <<<"${releases}" | newest_match "$1")"
    if [ -z "${version}" ]; then
        echo "No Temurin JDK release matches ${1#=}" >&2
        return 1
    fi
    link="$(awk -F'\t' -v version="${version}" '$1 == version {print $2; exit}' <<<"${releases}")"
    checksum="$(awk -F'\t' -v version="${version}" '$1 == version {print $3; exit}' <<<"${releases}")"
    home="${_jvm_root}/temurin-${version}"
    [ "${OS}" = "macos" ] && home="${home}.jdk"
    archive="$(mktemp)"
    log "Downloading Temurin JDK ${version}..."
    curl -fsSL -o "${archive}" "${link}"
    if [ "$(_sha256 "${archive}")" != "${checksum}" ]; then
        echo "Checksum mismatch for ${link}" >&2
        return 1
    fi
    ${_sudo} mkdir -p "${_jvm_root}"
    ${_sudo} rm -rf "${home}"
    ${_sudo} mkdir "${home}"
    ${_sudo} tar -C "${home}" --strip-components=1 -xzf "${archive}"
    rm -f "${archive}"
}

_install_versions() {
    local specs spec
    specs="$(missing_versions java)"
    while IFS= read -r spec; do
        [ -n "${spec}" ] || continue
        _install_jdk "${spec}"
    done <<<"${specs}"
}
deps() {
    _install_versions
    jvm_build_tools java
}

# With arguments, from the JVM languages' installers (jvm_jdks in
# _common.sh): only those JDKs, as feature versions ("17"), each the newest
# release of its line.
if [ "$#" -gt 0 ]; then
    for _feature in "$@"; do _install_jdk "${_feature}"; done
    exit 0
fi

log "Installing Java (Temurin JDK)..."
"install_${OS}"
deps

#!/usr/bin/env bash
# Install Node.js (TypeScript runtime)
source "$(dirname "$0")/../_common.sh"
# shellcheck source=languages/typescript/package-manager.sh
source "$(dirname "$0")/../typescript/package-manager.sh"

# Official builds, one directory per version, side by side.
if [ "${OS}" = "macos" ]; then
    _node_root="${HOME}/.local/share/hecatron/node"
    _sudo=""
else
    _node_root="/opt/node"
    _sudo="${SUDO}"
fi

install_macos() { brew_install node; }
# The default version's official build, linked into /usr/local/bin, which
# every shell has on PATH (distributions' Node.js lags, and NodeSource has
# only each line's newest release).
install_linux() {
    has node && return
    local version tool
    version="$(default_version)"
    _download_node "${version}"
    for tool in node npm npx corepack; do
        $SUDO ln -sfn "${_node_root}/${version}/bin/${tool}" "/usr/local/bin/${tool}"
    done
}
_download_node() {
    local os arch
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    case "$(uname -m)" in
        x86_64) arch="x64" ;;
        aarch64|arm64) arch="arm64" ;;
        *) arch="$(uname -m)" ;;
    esac
    log "Downloading Node.js $1 for ${os}-${arch}..."
    ${_sudo} rm -rf "${_node_root:?}/$1"
    ${_sudo} mkdir -p "${_node_root}/$1"
    curl -fsSL "https://nodejs.org/dist/v$1/node-v$1-${os}-${arch}.tar.xz" |
        ${_sudo} tar -C "${_node_root}/$1" --strip-components=1 -xJ
}

# Every other version the project's components need, resolved against
# nodejs.org's list of releases.
_releases() {
    curl -fsSL https://nodejs.org/dist/index.json |
        python3 -c 'import json, sys; [print(r["version"].lstrip("v")) for r in json.load(sys.stdin)]'
}
_install_versions() {
    local specs spec version
    # JavaScript's components run on the same Node.js.
    specs="$( (missing_versions typescript; missing_versions javascript) | sort -u)"
    while IFS= read -r spec; do
        [ -n "${spec}" ] || continue
        version="$(_releases | newest_match "${spec}")"
        if [ -z "${version}" ]; then
            echo "No Node.js release matches ${spec}" >&2
            return 1
        fi
        _download_node "${version}"
    done <<<"${specs}"
}

# Only libraries with a package.json have dependencies to install, with the
# package manager each declares.
_node_install() {
    [ -f package.json ] || return 0
    node_package_manager_ensure
    "${node_pm[@]}" install
}
deps() {
    _install_versions
    each_impl typescript _node_install
}

log "Installing TypeScript (Node.js)..."
"install_${OS}"
deps

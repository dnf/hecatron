#!/usr/bin/env bash
# Install the .NET SDK (shared by C#, F# and VB.NET): the default version and
# every version the project's components need, side by side in one directory
# with Microsoft's dotnet-install.sh. A distribution's package holds one
# feature band (Ubuntu's dotnet-sdk-10.0 is 10.0.1xx), and a newer analyzer
# or source generator needs a newer band's compiler. Within the directory,
# `dotnet` runs the SDK global.json asks for, else the newest, so each SDK
# also gets a view (_make_views) where it's the only one.
#
# F#'s and VB.NET's installers run this one with their language as $1, so
# their components' versions count.
source "$(dirname "$0")/../_common.sh"

_lang="${1:-csharp}"
if [ "${OS}" = "macos" ]; then
    _dotnet_root="${HOME}/.local/share/hecatron/dotnet"
    _sudo=""
else
    _dotnet_root="/opt/dotnet"
    _sudo="${SUDO}"
fi

install_macos() { _install_default; }
# ICU, which .NET needs and a minimal system lacks (named for its version:
# libicu74, libicu76...), and the default SDK, linked into /usr/local/bin,
# which every shell has on PATH, when there's no dotnet.
install_linux() {
    apt_install ca-certificates "$(_libicu)"
    _install_default
    has dotnet || $SUDO ln -sfn "${_dotnet_root}/dotnet" /usr/local/bin/dotnet
}
_libicu() {
    apt_refresh
    apt-cache pkgnames libicu | grep -E '^libicu[0-9]+$' | sort -V | tail -1
}
_install_default() { _install_sdk "=$(default_version)"; }

# Microsoft's dotnet-install.sh, downloaded once to a file rather than piped
# into bash, so a dropped connection (curl's error 56, which --retry alone
# doesn't cover) is retried and a partial script never runs. The script
# retries its own downloads.
_dotnet_install_script=""
_dotnet_install() {
    if [ -z "${_dotnet_install_script}" ]; then
        _dotnet_install_script="$(mktemp)"
        trap 'rm -f "${_dotnet_install_script}"' EXIT
        curl --retry-all-errors -fsSL -o "${_dotnet_install_script}" https://dot.net/v1/dotnet-install.sh
    fi
    ${_sudo} bash "${_dotnet_install_script}" "$@"
}

# Every SDK release of the .NET channels (8.0, 10.0...) a spec can match,
# from Microsoft's release metadata.
_sdk_releases() {
    local spec="${1#=}" channel
    curl -fsSL https://builds.dotnet.microsoft.com/dotnet/release-metadata/releases-index.json |
        python3 -c '
import json, sys
spec = sys.argv[1].split(".")
for channel in json.load(sys.stdin)["releases-index"]:
    version = channel["channel-version"].split(".")
    if version[:len(spec[:2])] == spec[:2]:
        print(channel["channel-version"])
' "${spec}" |
        while IFS= read -r channel; do
            curl -fsSL "https://builds.dotnet.microsoft.com/dotnet/release-metadata/${channel}/releases.json" |
                python3 -c '
import json, sys
for release in json.load(sys.stdin)["releases"]:
    for sdk in release.get("sdks") or [release["sdk"]]:
        print(sdk["version"])
'
        done
}
_has_sdk() { [ -d "${_dotnet_root}/sdk/$1" ]; }
_install_sdk() {
    local version
    if [[ "$1" == =* ]] && _has_sdk "${1#=}"; then return; fi
    version="$(_sdk_releases "$1" | newest_match "$1")"
    if [ -z "${version}" ]; then
        echo "No .NET SDK release matches ${1#=}" >&2
        return 1
    fi
    _has_sdk "${version}" && return
    log "Installing the .NET SDK ${version} into ${_dotnet_root}..."
    ${_sudo} mkdir -p "${_dotnet_root}"
    _dotnet_install --version "${version}" --install-dir "${_dotnet_root}" --no-path
}

_install_versions() {
    local specs spec
    specs="$(missing_versions "${_lang}")"
    while IFS= read -r spec; do
        [ -n "${spec}" ] || continue
        _install_sdk "${spec}"
    done <<<"${specs}"
}

# The .NET runtimes the components' projects target, so their tests can run
# (dotnet_runtimes_missing in _common.sh), into the same directory.
_install_runtime() {
    log "Installing the .NET $1 runtime into ${_dotnet_root}..."
    _dotnet_install --runtime dotnet --channel "$1.0" --install-dir "${_dotnet_root}" --no-path
}

# Each SDK's view, <root>-views/<version>: a directory where `dotnet` sees
# that SDK alone, which installed.sh gives as the SDK's home, so a component
# runs the SDK its version resolves to (docs/design.md §18) without a
# global.json. Everything else (runtimes, packs, templates, workload
# manifests) is the shared directory's, linked; the host is copied, since it
# finds its directory from its own real path. A view whose SDK is gone goes.
_views="${_dotnet_root}-views"
_make_views() {
    local sdk version view entry
    for sdk in "${_dotnet_root}"/sdk/*/; do
        [ -f "${sdk}dotnet.dll" ] || continue
        version="$(basename "${sdk}")"
        view="${_views}/${version}"
        ${_sudo} mkdir -p "${view}/sdk"
        cmp -s "${_dotnet_root}/dotnet" "${view}/dotnet" || ${_sudo} cp -f "${_dotnet_root}/dotnet" "${view}/dotnet"
        for entry in "${_dotnet_root}"/*; do
            case "$(basename "${entry}")" in
                dotnet | sdk) ;;
                *) ${_sudo} ln -sfn "${entry}" "${view}/" ;;
            esac
        done
        ${_sudo} ln -sfn "${sdk%/}" "${view}/sdk/${version}"
    done
    for view in "${_views}"/*/; do
        [ -d "${view}" ] && [ ! -d "${_dotnet_root}/sdk/$(basename "${view}")" ] && ${_sudo} rm -rf "${view}"
    done
    return 0
}

deps() {
    local _major
    _install_versions
    export DOTNET_ROOT="${_dotnet_root}"
    export PATH="${_dotnet_root}:${PATH}"
    while IFS= read -r _major; do
        [ -n "${_major}" ] && _install_runtime "${_major}"
    done < <(dotnet_runtimes_missing)
    _make_views
}

log "Installing the .NET SDK..."
"install_${OS}"
deps

#!/usr/bin/env bash
# Install Haskell (GHC + cabal via ghcup)
source "$(dirname "$0")/../_common.sh"

_install() {
    if ! has ghc; then
        curl --proto '=https' --tlsv1.2 -sSf https://get-ghcup.haskell.org | \
            BOOTSTRAP_HASKELL_NONINTERACTIVE=1 \
            BOOTSTRAP_HASKELL_INSTALL_CABAL=1 \
            BOOTSTRAP_HASKELL_INSTALL_GHC=1 \
            BOOTSTRAP_HASKELL_ADJUST_BASHRC=1 \
            sh
    fi
}

install_macos() { _install; }
# GHC's documented build dependencies, which a fresh Debian or Ubuntu lacks.
install_linux() {
    apt_install build-essential libffi-dev libgmp-dev libncurses-dev pkg-config zlib1g-dev
    _install
}

# shellcheck source=_tools.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_tools.sh"

# Stack through ghcup, which this shell's PATH may predate.
_install_stack() {
    local ghcup="${HOME}/.ghcup/bin/ghcup"
    has ghcup && ghcup="ghcup"
    "${ghcup}" install stack --set recommended
}
# HLint: the release build where there is one (x86_64 Linux), else the
# distribution's or Homebrew's.
HLINT_VERSION="3.10"
_install_hlint() {
    if [ "${OS}" = "macos" ]; then brew_install hlint; return; fi
    if [ "$(uname -m)" != "x86_64" ]; then apt_install hlint; return; fi
    log "Downloading HLint ${HLINT_VERSION}..."
    $SUDO rm -rf /opt/hlint
    $SUDO mkdir -p /opt/hlint
    curl -fsSL "https://github.com/ndmitchell/hlint/releases/download/v${HLINT_VERSION}/hlint-${HLINT_VERSION}-x86_64-linux.tar.gz" |
        $SUDO tar -C /opt/hlint --strip-components=1 -xz
    $SUDO ln -sfn /opt/hlint/hlint /usr/local/bin/hlint
}

# Fourmolu and Ormolu (hecatron format's Haskell formatters): their release
# builds, checked against their checksums, where there is one; else
# Homebrew's (macOS has no single-file Ormolu build).
_install_fourmolu() {
    local _platform _sha256
    case "${OS}-$(uname -m)" in
        linux-x86_64) _platform=linux-x86_64 _sha256=aadf6931653df7b96f52014cf8eb4285a3e8778cd0b531499fc3ee013e1e00c7 ;;
        linux-aarch64 | linux-arm64) _platform=linux-arm64 _sha256=8ef665cab33041a9ef702a4f6d7879b0e5988a71eb790fd0841c5c8d07789988 ;;
        macos-x86_64) _platform=darwin-x86_64 _sha256=4799cf43ced9b9b63b01879df110ec67c89c8d3a7cee0188427c17fe388e2f10 ;;
        macos-arm64) _platform=darwin-arm64 _sha256=e62da50e4fafc9b0a2ec437192b13b690f0d6ea663d18d97cc14a40c30f7a04c ;;
        *) echo "WARNING: no Fourmolu release for $(uname -sm)" >&2; return 0 ;;
    esac
    log "Installing Fourmolu ${_FOURMOLU_VERSION} (for hecatron format)..."
    release_binary_install fourmolu \
        "https://github.com/fourmolu/fourmolu/releases/download/v${_FOURMOLU_VERSION}/fourmolu-${_FOURMOLU_VERSION}-${_platform}.zip" \
        "${_sha256}" "fourmolu-${_FOURMOLU_VERSION}-${_platform}/fourmolu"
}
_install_ormolu() {
    if [ "${OS}-$(uname -m)" != linux-x86_64 ]; then
        if [ "${OS}" = macos ]; then brew_install ormolu; else echo "WARNING: no Ormolu release for $(uname -sm)" >&2; fi
        return 0
    fi
    log "Installing Ormolu ${_ORMOLU_VERSION} (for hecatron format)..."
    release_binary_install ormolu \
        "https://github.com/tweag/ormolu/releases/download/${_ORMOLU_VERSION}/ormolu-x86_64-linux.zip" \
        367eb4969986504cf171f71f91e24ea5d7d9d97c41701e8e6eef9b642fd0d201
}

deps() {
    local _tool
    while IFS= read -r _tool; do
        case "${_tool}" in
            stack) _install_stack ;;
            hlint) _install_hlint ;;
            fourmolu) _install_fourmolu ;;
            ormolu) _install_ormolu ;;
        esac
    done < <(haskell_tools_missing)
}

log "Installing Haskell..."
"install_${OS}"
deps
# The dependency auditor hecatron audit uses for this language.
osv_scanner_install

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

deps() {
    local _tool
    while IFS= read -r _tool; do
        case "${_tool}" in
            stack) _install_stack ;;
            hlint) _install_hlint ;;
        esac
    done < <(haskell_tools_missing)
}

log "Installing Haskell..."
"install_${OS}"
deps

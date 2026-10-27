#!/usr/bin/env bash
# Install Elm (requires Node.js)
source "$(dirname "$0")/../_common.sh"

install_macos() {
    # Ensure Node.js is available
    if ! has node; then
        bash "$(dirname "$0")/../typescript/install.sh"
    fi
    _npm_global elm@latest elm-test@latest
}

install_linux() {
    if ! has node; then
        bash "$(dirname "$0")/../typescript/install.sh"
    fi
    ARCH="$(dpkg --print-architecture)"
    if [ "$ARCH" = "arm64" ] || [ "$ARCH" = "aarch64" ]; then
        echo "WARNING: Elm does not provide pre-built arm64 Linux binaries." >&2
        echo "See https://github.com/elm/compiler/issues/2292 for details." >&2
        echo "Skipping Elm installation on arm64." >&2
        return 1
    fi
    _npm_global elm@latest elm-test@latest
}

# Global npm packages go into Node's prefix, which may be root's. An official
# Node.js build's prefix is its own directory (/opt/node/<version>), off PATH,
# so link the commands into /usr/local/bin.
_npm_global() {
    local prefix tool
    prefix="$(npm prefix -g)"
    if [ -w "${prefix}/lib" ]; then
        npm install -g "$@"
    else
        $SUDO npm install -g "$@"
    fi
    for tool in elm elm-test; do
        has "${tool}" || $SUDO ln -sfn "${prefix}/bin/${tool}" "/usr/local/bin/${tool}"
    done
}

# elm-review where a component configures it (review/elm.json).
_ELM_REVIEW_VERSION="2.13.5"
deps() {
    if elm_review_missing; then _npm_global "elm-review@${_ELM_REVIEW_VERSION}"; fi
}

log "Installing Elm..."
"install_${OS}"
deps

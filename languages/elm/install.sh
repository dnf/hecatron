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
    # Every command the install added (elm, elm-test, elm-review...) that
    # isn't on PATH yet.
    for tool in "${prefix}"/bin/*; do
        [ -e "${tool}" ] || continue
        has "$(basename "${tool}")" || $SUDO ln -sfn "${tool}" "/usr/local/bin/$(basename "${tool}")"
    done
}

# elm-review where a component configures it (review/elm.json).
_ELM_REVIEW_VERSION="2.13.5"
# elm-format, which hecatron format runs on every component.
_ELM_FORMAT_VERSION="0.8.8"
deps() {
    if elm_review_missing; then _npm_global "elm-review@${_ELM_REVIEW_VERSION}"; fi
    if elm_format_missing; then _npm_global "elm-format@${_ELM_FORMAT_VERSION}"; fi
}

log "Installing Elm..."
"install_${OS}"
deps

#!/usr/bin/env bash
# Install the Pharo Smalltalk VM (zeroconf) into ~/.pharo, with `pharo` on PATH
source "$(dirname "$0")/../_common.sh"

PHARO_VERSION="120"
PHARO_HOME="${HOME}/.pharo"
# ~/.local/bin rather than /usr/local/bin, which Apple Silicon Macs don't have
# and which needs root; the launcher follows the symlink back to its VM.
BIN_DIR="${HOME}/.local/bin"

_install() {
    if has pharo; then
        log "Pharo already installed"
        return
    fi
    mkdir -p "${PHARO_HOME}" "${BIN_DIR}"
    (cd "${PHARO_HOME}" && curl -fsSL "https://get.pharo.org/64/vm${PHARO_VERSION}" | bash)
    ln -sf "${PHARO_HOME}/pharo" "${BIN_DIR}/pharo"
    case ":${PATH}:" in
        *":${BIN_DIR}:"*) ;;
        *) log "Add ${BIN_DIR} to your PATH to use pharo" ;;
    esac
}

install_macos() { _install; }
install_linux() { _install; }

# The test image is downloaded into age/impl/pharo by test.sh on first run.
deps() { :; }

log "Installing Pharo..."
"install_${OS}"
deps

#!/usr/bin/env bash
# Install SBCL (Steel Bank Common Lisp), and Quicklisp in hecatron's data
# directory (no root), which the hooks fetch a system's dependencies with
# (as install.json's Quicklisp step does).
source "$(dirname "$0")/../_common.sh"

_quicklisp_url="https://beta.quicklisp.org/quicklisp.lisp"
# Of the quickstart signed with Quicklisp's release key
# (D7A3489DDEFE32B7D0E7CC61307965AB028B5FF7, quicklisp.lisp.asc).
_quicklisp_sha256="4a7a5c2aebe0716417047854267397e24a44d0cce096127411e9ce9ccfeb2c17"

install_macos() { brew_install sbcl; }
install_linux() { apt_install sbcl; }

# Quicklisp's quickstart refuses a directory it was installed in before, so
# an interrupted install's leftovers are removed first.
install_quicklisp() {
    local _home _work
    _home="$(hecatron_data)/quicklisp"
    [ ! -f "${_home}/setup.lisp" ] || return 0
    log "Installing Quicklisp into ${_home}..."
    _work="$(mktemp -d)"
    curl -fsSL -o "${_work}/quicklisp.lisp" "${_quicklisp_url}"
    if [ "$(sha256_of "${_work}/quicklisp.lisp")" != "${_quicklisp_sha256}" ]; then
        rm -rf "${_work}"
        echo "error: the Quicklisp download doesn't match its checksum (${_quicklisp_url})" >&2
        return 1
    fi
    rm -rf "${_home}"
    sbcl --noinform --non-interactive --no-sysinit --no-userinit --load "${_work}/quicklisp.lisp" \
        --eval "(quicklisp-quickstart:install :path \"${_home}/\")"
    rm -rf "${_work}"
}

log "Installing SBCL..."
"install_${OS}"
install_quicklisp

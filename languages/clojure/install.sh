#!/usr/bin/env bash
# Install Clojure CLI (on the JDK Java installs), and Leiningen where a
# component builds with project.clj
source "$(dirname "$0")/../_common.sh"

_LEIN_VERSION="2.12.0"

install_macos() { brew_install clojure/tools/clojure; }
install_linux() {
    has java || bash "$(dirname "$0")/../java/install.sh"
    if ! has clojure; then
        curl -fsSL https://download.clojure.org/install/linux-install.sh | $SUDO bash
    fi
}

# The components that build with Leiningen, one per line.
lein_dirs() {
    local _dir
    while IFS= read -r _dir; do
        [ -f "${_dir}/project.clj" ] && echo "${_dir}"
    done < <(impl_dirs clojure)
    return 0
}
# Leiningen's script fetches its own jar on first run.
_install_lein() {
    local _script
    log "Installing Leiningen ${_LEIN_VERSION}..."
    if [ "${OS}" = "macos" ]; then
        brew_install leiningen
    else
        _script="$(mktemp)"
        curl -fsSL -o "${_script}" "https://raw.githubusercontent.com/technomancy/leiningen/${_LEIN_VERSION}/bin/lein"
        $SUDO install -m 0755 "${_script}" /usr/local/bin/lein
        rm -f "${_script}"
    fi
    lein version
}
deps() {
    if [ -n "$(lein_dirs)" ] && ! has lein; then _install_lein; fi
}

log "Installing Clojure..."
"install_${OS}"
deps

#!/usr/bin/env bash
# Install SWI-Prolog: on Ubuntu from SWI-Prolog's PPA, whose stable release
# is newer than Ubuntu's (packs such as lsp_server need it); elsewhere the
# distribution's (as install.json's "aptSources" says).
source "$(dirname "$0")/../_common.sh"

swi_prolog_ppa() {
    apt_source swi-prolog ubuntu https://ppa.launchpadcontent.net/swi-prolog/stable/ubuntu \
        "https://keyserver.ubuntu.com/pks/lookup?op=get&search=0xE8B739E3753FF4A12360BA6A4AB3A5F60EA9AEB3" \
        E8B739E3753FF4A12360BA6A4AB3A5F60EA9AEB3 main
}

install_macos() { brew_install swi-prolog; }
# Ubuntu's own SWI-Prolog, there before the PPA is added: its
# swi-prolog-core holds files the PPA's swi-prolog-nox takes over without
# declaring it, so dpkg refuses the upgrade until it's removed. Removed
# before the PPA's package lists are fetched, so apt removes what depends
# on it rather than upgrading that over it.
_ubuntu_packages=(swi-prolog-core swi-prolog-core-packages)

install_linux() {
    local _package _installed=()
    if apt_source_missing swi-prolog ubuntu; then
        for _package in "${_ubuntu_packages[@]}"; do
            if [[ "$(dpkg-query -W -f='${Status}' "${_package}" 2>/dev/null)" == *" installed" ]]; then
                _installed+=("${_package}")
            fi
        done
        if [ "${#_installed[@]}" -gt 0 ]; then $SUDO apt-get remove -y "${_installed[@]}"; fi
    fi
    swi_prolog_ppa || true
    apt_install swi-prolog
}

deps() { :; }

log "Installing Prolog (SWI-Prolog)..."
"install_${OS}"
deps

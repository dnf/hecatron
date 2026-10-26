#!/usr/bin/env bash
# Install PHP + Composer
source "$(dirname "$0")/../_common.sh"

install_macos() { brew_install php composer; }
install_linux() {
    apt_install php-cli php-xml php-mbstring
    if ! has composer; then
        curl -sS https://getcomposer.org/installer | $SUDO php -- --install-dir=/usr/local/bin --filename=composer
    fi
}

deps() { each_impl php composer install; }

log "Installing PHP..."
"install_${OS}"
deps

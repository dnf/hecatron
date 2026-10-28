#!/usr/bin/env bash
# Uninstall PHP: the packages install.sh installs for each PHP line (and the
# unversioned ones earlier versions installed), its links in /opt/php, and
# Composer. What apt pulled in with them (php8.4-common...) stays, since
# another package may need it; `apt-get autoremove` removes it once nothing
# does.
source "$(dirname "$0")/../_common.sh"

uninstall_macos() { brew uninstall php composer 2>/dev/null || true; }

# php8.4-cli, php8.4-xml, php8.4-mbstring, php-cli...: installed, by name.
_installed_php_packages() {
    dpkg-query -W -f='${db:Status-Abbrev} ${Package}\n' 'php*' 2>/dev/null |
        awk '$1 == "ii" { print $2 }' |
        grep -E '^php([0-9]+\.[0-9]+)?-(cli|xml|mbstring)$' || true
}
uninstall_linux() {
    local packages
    packages="$(_installed_php_packages)"
    if [ -n "${packages}" ]; then
        # shellcheck disable=SC2086 # one package name per word
        $SUDO apt-get remove -y ${packages} 2>/dev/null || true
    fi
    $SUDO rm -f /usr/local/bin/composer
    $SUDO rm -rf /opt/php
}

log "Uninstalling PHP..."
"uninstall_${OS}"

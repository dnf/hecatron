#!/usr/bin/env bash
# Uninstall Elixir: install.sh's Elixirs, the OTPs it installed for them and
# its wrappers, and the packages earlier versions installed.
source "$(dirname "$0")/../_common.sh"

uninstall_macos() {
    rm -rf "${HOME}/.local/share/hecatron/elixir"
    brew uninstall elixir 2>/dev/null || true
}
uninstall_linux() {
    local tool
    for tool in elixir elixirc iex mix; do
        if grep -q "hecatron's default Elixir" "/usr/local/bin/${tool}" 2>/dev/null; then
            $SUDO rm -f "/usr/local/bin/${tool}"
        fi
    done
    $SUDO rm -rf /opt/elixir /opt/erlang
    $SUDO apt-get remove -y elixir 2>/dev/null || true
}

log "Uninstalling Elixir..."
"uninstall_${OS}"

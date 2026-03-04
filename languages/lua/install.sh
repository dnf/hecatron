#!/usr/bin/env bash
# Install Lua + LuaRocks + busted + luacheck
source "$(dirname "$0")/../_common.sh"

install_macos() {
    brew_install lua luarocks
}
install_linux() {
    apt_install lua5.4 liblua5.4-dev luarocks
}

deps() {
    luarocks install busted
    luarocks install luacheck
}

log "Installing Lua..."
"install_${OS}"
deps

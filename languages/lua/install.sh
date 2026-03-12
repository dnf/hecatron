#!/usr/bin/env bash
# Install Lua + LuaRocks + busted + luacheck
source "$(dirname "$0")/../_common.sh"

install_macos() {
    brew_install lua luarocks
}
install_linux() {
    # Install the latest Lua available in the distro
    _lua_pkg=""
    for _v in lua5.5 lua5.4 lua5.3; do
        if apt-cache show "${_v}" &>/dev/null; then
            _lua_pkg="${_v}"
            break
        fi
    done
    if [ -z "${_lua_pkg}" ]; then
        log "No lua5.x package found — installing 'lua' meta-package"
        _lua_pkg="lua"
    fi
    _lua_dev="${_lua_pkg/lua/liblua}-dev"
    apt_install "${_lua_pkg}" "${_lua_dev}" luarocks
}

deps() {
    luarocks install busted
    luarocks install luacheck
}

log "Installing Lua..."
"install_${OS}"
deps

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
        if apt_has "${_v}"; then
            _lua_pkg="${_v}"
            break
        fi
    done
    if [ -z "${_lua_pkg}" ]; then
        echo "error: no lua5.3, 5.4 or 5.5 package in this distribution's apt sources" >&2
        return 1
    fi
    _lua_dev="${_lua_pkg/lua/liblua}-dev"
    apt_install "${_lua_pkg}" "${_lua_dev}" luarocks
}

# luacheck 1.2 can't run on Lua 5.5 (it assigns to loop variables, which 5.5
# makes constant), so it runs on Lua 5.4 whatever the default Lua is. The code
# it checks is unaffected. A luacheck that fails to start is reinstalled.
# Rocks go into ~/.luarocks: the system tree (/usr/local, from apt's luarocks
# or Homebrew's on Intel Macs) isn't writable by a normal user.
_luarocks() { luarocks --local "$@"; }
_install_luacheck() {
    luacheck --version >/dev/null 2>&1 && return
    local _lua54_dir=""
    if [ "${OS}" = "macos" ]; then
        brew_install lua@5.4
        _lua54_dir="$(brew --prefix lua@5.4)"
    else
        apt_install lua5.4 liblua5.4-dev
        _lua54_dir="/usr"
    fi
    local _rock
    for _rock in argparse luafilesystem luacheck; do
        _luarocks --lua-version=5.4 --lua-dir="${_lua54_dir}" install --force "${_rock}"
    done
}

deps() {
    _luarocks install busted
    _install_luacheck
}

log "Installing Lua..."
"install_${OS}"
deps

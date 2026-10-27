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
    if stylua_missing; then _install_stylua; fi
}
# StyLua (hecatron format's Lua formatter), its release binary.
_install_stylua() {
    local _platform _sha256
    case "${OS}-$(uname -m)" in
        linux-x86_64) _platform=linux-x86_64 _sha256=a485f72bbfa22e4642e77a84b5edc2611a5cab7477b34e2e709dd8f78e919895 ;;
        linux-aarch64 | linux-arm64) _platform=linux-aarch64 _sha256=8e67539e5fdf7a81cd03cf592b0df18d3bb96ee17a5ea5d9bf67b86387b903b0 ;;
        macos-x86_64) _platform=macos-x86_64 _sha256=90779aa873837b2dff8eb3361bd27950303914c24265a59cae0a0d0629d71585 ;;
        macos-arm64) _platform=macos-aarch64 _sha256=dab9b66d340ca5ffc8225211f1dcf89b84783e09b8cbec8c518497eba81f3c42 ;;
        *) echo "WARNING: no StyLua release for $(uname -sm)" >&2; return 0 ;;
    esac
    log "Installing StyLua ${_STYLUA_VERSION} (for hecatron format)..."
    release_binary_install stylua \
        "https://github.com/JohnnyMorganz/StyLua/releases/download/v${_STYLUA_VERSION}/stylua-${_platform}.zip" "${_sha256}"
}

log "Installing Lua..."
"install_${OS}"
deps

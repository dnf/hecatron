#!/usr/bin/env bash
# Check that busted and luacheck start (luacheck needs Lua 5.4; see
# install.sh), and for StyLua where a component configures it.
source "$(dirname "$0")/../_common.sh"

busted --version >/dev/null 2>&1 || problem "busted fails to start"
if has luacheck && ! luacheck --version >/dev/null 2>&1; then
    problem "luacheck fails to start"
fi
if stylua_missing; then problem "StyLua ${_STYLUA_VERSION} not installed (a component has a stylua.toml)"; fi
doctor_done

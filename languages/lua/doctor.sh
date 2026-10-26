#!/usr/bin/env bash
# Check that busted and luacheck start (luacheck needs Lua 5.4; see install.sh).
source "$(dirname "$0")/../_common.sh"

busted --version >/dev/null 2>&1 || problem "busted fails to start"
if has luacheck && ! luacheck --version >/dev/null 2>&1; then
    problem "luacheck fails to start"
fi
doctor_done

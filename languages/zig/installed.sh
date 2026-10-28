#!/usr/bin/env bash
# The Zigs installed on this machine, as "version<TAB>home" (home holds the
# zig command and its lib/): hecatron's downloads, then a zig on PATH
# installed some other way (an older installer's /opt/zig, a Dockerfile,
# Homebrew). Offline.
source "$(dirname "$0")/../_common.sh"

toolchain_installed zig zig
_zig="$(command -v zig 2>/dev/null || true)"
case "${_zig}" in
    "" | "$(hecatron_data)"/*) ;;
    *)
        _version="$(zig version 2>/dev/null)"
        [ -n "${_version}" ] && printf '%s\t%s\n' "${_version}" "$(dirname "${_zig}")"
        ;;
esac
exit 0

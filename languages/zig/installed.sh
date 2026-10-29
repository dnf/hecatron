#!/usr/bin/env bash
# The Zigs installed on this machine, as "version<TAB>home" (home holds the
# zig command and its lib/): hecatron's downloads, a Dockerfile's
# /opt/zig/<version>, then a zig on PATH installed some other way (an older
# installer's /opt/zig, Homebrew). Offline.
source "$(dirname "$0")/../_common.sh"

toolchain_installed zig zig
for _home in /opt/zig/*; do
    [ -x "${_home}/zig" ] && printf '%s\t%s\n' "${_home##*/}" "${_home}"
done
_zig="$(command -v zig 2>/dev/null || true)"
_real="$(readlink -f "${_zig}" 2>/dev/null || true)"
case "${_zig}" in
    "" | "$(hecatron_data)"/*) ;;
    *)
        case "${_real}" in
            /opt/zig/*/zig) exit 0 ;;
        esac
        _version="$(zig version 2>/dev/null)"
        [ -n "${_version}" ] && printf '%s\t%s\n' "${_version}" "$(dirname "${_zig}")"
        ;;
esac
exit 0

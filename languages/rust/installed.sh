#!/usr/bin/env bash
# The Rust toolchains installed on this machine, as "version<TAB>home" (home is
# the toolchain's sysroot, with bin/rustc): every rustup toolchain, and the
# rustc on PATH, which needn't be rustup's (Homebrew's, a distribution's).
# Offline.
source "$(dirname "$0")/../_common.sh"

_version() { "$@" --version 2>/dev/null | awk '{print $2}'; }
_toolchain() {
    local version home
    version="$(_version "$@")"
    home="$("$@" --print sysroot 2>/dev/null)"
    if [ -n "${version}" ] && [ -n "${home}" ]; then printf '%s\t%s\n' "${version}" "${home}"; fi
}

{
    if has rustup; then
        rustup toolchain list 2>/dev/null | awk '{print $1}' | while IFS= read -r toolchain; do
            _toolchain rustup run "${toolchain}" rustc
        done
    fi
    if has rustc; then _toolchain rustc; fi
} | sort -u

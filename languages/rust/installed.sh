#!/usr/bin/env bash
# The Rust toolchains installed on this machine, as "version<TAB>home" (home is
# the toolchain's sysroot, with bin/rustc): every rustup toolchain, else the
# rustc on PATH. Offline.
source "$(dirname "$0")/../_common.sh"

_version() { "$@" --version 2>/dev/null | awk '{print $2}'; }

if has rustup; then
    rustup toolchain list 2>/dev/null | awk '{print $1}' | while IFS= read -r toolchain; do
        version="$(_version rustup run "${toolchain}" rustc)"
        home="$(rustup run "${toolchain}" rustc --print sysroot 2>/dev/null)"
        [ -n "${version}" ] && [ -n "${home}" ] && printf '%s\t%s\n' "${version}" "${home}"
    done | sort -u
elif has rustc; then
    printf '%s\t%s\n' "$(_version rustc)" "$(rustc --print sysroot)"
fi

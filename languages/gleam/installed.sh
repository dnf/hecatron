#!/usr/bin/env bash
# The Gleams installed on this machine, as "version<TAB>home" (home holds
# the gleam command): hecatron's downloads, then a gleam on PATH installed
# some other way (an older installer's /usr/local/bin, a Dockerfile,
# Homebrew). Offline.
source "$(dirname "$0")/../_common.sh"

toolchain_installed gleam gleam
_gleam="$(command -v gleam 2>/dev/null || true)"
case "${_gleam}" in
    "" | "$(hecatron_data)"/*) ;;
    *)
        _version="$(gleam --version 2>/dev/null | sed -n 's/^gleam //p')"
        [ -n "${_version}" ] && printf '%s\t%s\n' "${_version}" "$(dirname "${_gleam}")"
        ;;
esac
exit 0

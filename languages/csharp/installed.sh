#!/usr/bin/env bash
# The .NET SDKs installed on this machine, as "version<TAB>home" (home is the
# directory with the dotnet command, its DOTNET_ROOT): hecatron's
# (install.sh), the one the dotnet on PATH belongs to, and the usual places
# for distributions' packages, Microsoft's installers and Homebrew. A
# directory holds many SDKs, in sdk/<version>. Offline.
source "$(dirname "$0")/../_common.sh"

_roots() {
    local dotnet root
    dotnet="$(command -v dotnet 2>/dev/null || true)"
    [ -n "${dotnet}" ] && dirname "$(python3 -c 'import os, sys; print(os.path.realpath(sys.argv[1]))' "${dotnet}")"
    for root in /opt/dotnet \
                "${HOME}/.local/share/hecatron/dotnet" \
                "${HOME}/.dotnet" \
                /usr/lib/dotnet \
                /usr/share/dotnet \
                /usr/local/share/dotnet \
                /opt/homebrew/opt/dotnet/libexec \
                /usr/local/opt/dotnet/libexec; do
        [ -x "${root}/dotnet" ] && echo "${root}"
    done
    return 0
}

_roots | while IFS= read -r root; do
    for sdk in "${root}"/sdk/*/; do
        [ -f "${sdk}dotnet.dll" ] && printf '%s\t%s\n' "$(basename "${sdk}")" "${root}"
    done
done | sort -u

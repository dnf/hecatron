#!/usr/bin/env bash
# The Node.js versions installed on this machine, as "version<TAB>home" (home
# holds bin/node): the one on PATH, hecatron's official builds (install.sh),
# nvm's, and Homebrew's versioned formulas. Offline.
source "$(dirname "$0")/../_common.sh"

_candidates() {
    command -v node 2>/dev/null || true
    local node
    for node in /opt/node/*/bin/node \
                "${HOME}"/.local/share/hecatron/node/*/bin/node \
                "${HOME}"/.nvm/versions/node/*/bin/node \
                /opt/homebrew/opt/node@*/bin/node \
                /usr/local/opt/node@*/bin/node; do
        [ -x "${node}" ] && echo "${node}"
    done
    return 0
}

_candidates | while IFS= read -r node; do
    version="$("${node}" --version 2>/dev/null)" || continue
    home="$(cd "$(dirname "$(python3 -c 'import os, sys; print(os.path.realpath(sys.argv[1]))' "${node}")")/.." && pwd)"
    printf '%s\t%s\n' "${version#v}" "${home}"
done | sort -u

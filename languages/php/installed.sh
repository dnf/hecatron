#!/usr/bin/env bash
# The PHPs installed on this machine, as "version<TAB>home" (home holds
# bin/php for that version): install.sh's directories per line, Homebrew's
# kegs, and the php on PATH when it's a version neither has. Offline.
source "$(dirname "$0")/../_common.sh"

_candidates() {
    local php
    for php in /opt/php/*/bin/php \
               /opt/homebrew/opt/php*/bin/php \
               /usr/local/opt/php*/bin/php; do
        [ -x "${php}" ] && echo "${php}"
    done
    command -v php 2>/dev/null || true
}

# Homes as found, not resolved: /opt/php/8.4/bin/php links to /usr/bin/php8.4,
# whose directory's php is another version.
_candidates | while IFS= read -r php; do
    version="$("${php}" -r 'echo PHP_VERSION;' 2>/dev/null)" || continue
    printf '%s\t%s\n' "${version}" "$(cd "$(dirname "${php}")/.." && pwd)"
done | awk -F'\t' '!seen[$1]++'

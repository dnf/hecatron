#!/usr/bin/env bash
# The Rubies installed on this machine, as "version<TAB>home" (home holds
# bin/ruby): the one on PATH, hecatron's (install.sh), and rbenv's, chruby's
# and Homebrew's. Offline.
source "$(dirname "$0")/../_common.sh"

_candidates() {
    command -v ruby 2>/dev/null || true
    local ruby
    for ruby in /opt/ruby/*/bin/ruby \
                "${HOME}"/.local/share/hecatron/ruby/*/bin/ruby \
                "${HOME}"/.rbenv/versions/*/bin/ruby \
                "${HOME}"/.rubies/*/bin/ruby \
                /opt/homebrew/opt/ruby*/bin/ruby \
                /usr/local/opt/ruby*/bin/ruby; do
        [ -x "${ruby}" ] && echo "${ruby}"
    done
    return 0
}

_candidates | while IFS= read -r ruby; do
    version="$("${ruby}" -e 'print RUBY_VERSION' 2>/dev/null)" || continue
    home="$(cd "$(dirname "$(python3 -c 'import os, sys; print(os.path.realpath(sys.argv[1]))' "${ruby}")")/.." && pwd)"
    printf '%s\t%s\n' "${version}" "${home}"
done | sort -u

#!/usr/bin/env bash
# The Elixirs install.sh installed, as "version<TAB>home" (home holds
# bin/elixir, its OTP linked as erlang/ and its Hex in archives/). Only
# hecatron's: another Elixir (the distribution's, Homebrew's) has no OTP
# paired with it or Hex of its own. Offline: each release's VERSION file.
source "$(dirname "$0")/../_common.sh"

for home in /opt/elixir/*/ "${HOME}"/.local/share/hecatron/elixir/*/; do
    [ -x "${home}bin/elixir" ] && [ -f "${home}VERSION" ] || continue
    printf '%s\t%s\n' "$(tr -d '[:space:]' <"${home}VERSION")" "${home%/}"
done | sort -u

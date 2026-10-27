#!/usr/bin/env bash
# Check for the ocamlformat a component configures (see install.sh).
source "$(dirname "$0")/../_common.sh"
# shellcheck source=_ocamlformat.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_ocamlformat.sh"

has opam && eval "$(opam env 2>/dev/null)"
while IFS= read -r _version; do
    problem "ocamlformat ${_version} not installed (a component's .ocamlformat asks for it)"
done < <(ocamlformat_missing)
doctor_done

#!/usr/bin/env bash
# Default build hook: each package (a directory with an __init__.mojo, in src/
# when there is one) into build/<name>.mojopkg (see _mojo.sh).
# shellcheck source=_mojo.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_mojo.sh"
_packages="$(mojo_packages)"
[ -n "${_packages}" ] || unknown "no package (a directory with an __init__.mojo) in $(mojo_source_root)"
mkdir -p "${_mojo_build_dir}"
while IFS= read -r _package; do
    run mojo package "${_package}" -o "${_mojo_build_dir}/$(basename "${_package}").mojopkg"
done <<< "${_packages}"

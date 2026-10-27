#!/usr/bin/env bash
# Default lint hook: fail where mojo format would change a file. It has no
# check mode, so it formats copies and compares (see _mojo.sh).
# shellcheck source=_mojo.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_mojo.sh"
_files="$(mojo_files)"
[ -n "${_files}" ] || unknown "no .mojo files"
_copies="$(mktemp -d)"
trap 'rm -rf "${_copies}"' EXIT
while IFS= read -r _file; do
    mkdir -p "${_copies}/$(dirname "${_file}")"
    cp "${_file}" "${_copies}/${_file}"
done <<< "${_files}"
echo "+ mojo format (checking, on copies)"
mojo format --quiet "${_copies}"
_unformatted=0
while IFS= read -r _file; do
    diff -u "${_file}" "${_copies}/${_file}" || _unformatted=1
done <<< "${_files}"
[ "${_unformatted}" -eq 0 ] || { echo "error: mojo: run mojo format on the files above" >&2; exit 1; }

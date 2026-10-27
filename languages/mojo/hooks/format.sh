#!/usr/bin/env bash
# Default format hook: mojo format (see _mojo.sh).
# shellcheck source=_mojo.sh source-path=SCRIPTDIR
source "$(dirname "$0")/_mojo.sh"
if format_checking; then mojo_format_check; exit; fi
_files=()
while IFS= read -r _file; do _files+=("${_file}"); done < <(mojo_files)
[ "${#_files[@]}" -gt 0 ] || unknown "no .mojo files"
run mojo format "${_files[@]}"

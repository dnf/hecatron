#!/usr/bin/env bash
# Builds in other languages call `python` (a Rust crate's build script, a
# CMake project's), which many systems don't have beside python3. Without
# one on PATH, a `python` link to python3 goes in a directory of Python's
# own, a static "path" entry of its definition: every language's hooks have
# it, after their own toolchains, and nothing outside hecatron's hooks sees
# it. Once there is another `python`, the link goes. Run by install.sh.
set -euo pipefail

_links="${HOME}/.local/share/hecatron/python/bin"

# PATH without the links directory.
_path_without_links() {
    local _dir _kept="" IFS=:
    for _dir in ${PATH}; do
        [ "${_dir}" = "${_links}" ] || _kept="${_kept:+${_kept}:}${_dir}"
    done
    echo "${_kept}"
}

if PATH="$(_path_without_links)" command -v python >/dev/null 2>&1; then
    rm -f "${_links}/python"
    exit 0
fi
_python3="$(command -v python3)" || exit 0
mkdir -p "${_links}"
ln -sf "${_python3}" "${_links}/python"

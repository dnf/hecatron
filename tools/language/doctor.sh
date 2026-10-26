#!/usr/bin/env bash
# Check that each language's toolchain works, and say how to repair it.
#
# Usage: doctor.sh [--fix] [lang ...]   (default: every added, enabled language)
#
# A language is healthy when the build and lint dependencies named in its
# config.json run, and its optional .hecatron/languages/<lang>/doctor.sh
# passes. A doctor.sh prints one line per problem and exits non-zero when it
# finds any. With --fix, an unhealthy language's install.sh runs (installers
# repair the states their doctor.sh detects), and the language is checked again.
set -euo pipefail

source "$(dirname "$0")/helpers.sh"

_fix=false
_langs=()
for _arg in "$@"; do
    if [ "${_arg}" = "--fix" ]; then
        _fix=true
    else
        _langs+=("$(_name "${_arg}")")
    fi
done
if [ "${#_langs[@]}" -eq 0 ]; then
    for _lang in ${_ADDED_LANGS}; do
        _is_disabled "${_lang}" || _langs+=("${_lang}")
    done
fi

# Problems with one language, one per line; nothing when it is healthy.
_problems() {
    local _lang="$1" _dep _lint _script
    _dep="$(_dep "${_lang}")"
    _lint="$(_lint_dep "${_lang}")"
    if [ -n "${_dep}" ] && ! command -v "${_dep}" >/dev/null 2>&1; then
        echo "${_dep} not found"
        return
    fi
    if [ -n "${_lint}" ] && ! command -v "${_lint}" >/dev/null 2>&1; then
        echo "lint tool ${_lint} not found"
    fi
    _script="${_ROOT}/.hecatron/languages/${_lang}/doctor.sh"
    if [ -f "${_script}" ]; then
        (cd "${_ROOT}" && bash "${_script}") || true
    fi
}

_unhealthy=()
for _lang in "${_langs[@]}"; do
    # Only Ada gets Alire's toolchain on PATH (it would shadow the system gcc).
    if [ "${_lang}" = "ada" ]; then _ada_env; fi
    _found="$(_problems "${_lang}")"
    if [ -n "${_found}" ] && ${_fix}; then
        echo "${_lang}: repairing with .hecatron/languages/${_lang}/install.sh"
        bash "${_ROOT}/.hecatron/languages/${_lang}/install.sh" || true
        _found="$(_problems "${_lang}")"
    fi
    if [ -z "${_found}" ]; then
        printf '  ok    %s\n' "${_lang}"
    else
        printf '  FAIL  %s\n' "${_lang}"
        printf '%s\n' "${_found}" | sed 's/^/          /'
        _unhealthy+=("${_lang}")
    fi
done

echo
if [ "${#_unhealthy[@]}" -eq 0 ]; then
    echo "All ${#_langs[@]} toolchains look healthy."
    exit 0
fi
echo "${#_unhealthy[@]} of ${#_langs[@]} need attention: ${_unhealthy[*]}"
${_fix} || echo "Repair with: just doctor --fix ${_unhealthy[*]}   (runs each language's installer)"
exit 1

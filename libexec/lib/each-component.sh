#!/usr/bin/env bash
# Run one of a language's hooks (test, lint, build, clean, audit, format, deps,
# outdated, docs or bench) for every component, or the ones named. Lint also
# runs the format hook's check (docs/guide/format.md).
#
# Usage: each-component.sh <action> <lang> [component ...]
#
# The hook is .hecatron/languages/<lang>/<action>.sh in the project, else
# hecatron's built-in default (docs/guide/configuration.md, docs/guide/hooks.md). It runs from inside the
# component's directory (config.json "implPath") with the component name as
# $1, and with HECATRON_ROOT, HECATRON_COMPONENT and HECATRON_LANG set. Every
# component runs even after one fails, so one run reports all of them.
set -euo pipefail

# shellcheck source=helpers.sh
source "${HECATRON_HOME}/libexec/lib/helpers.sh"

_action="${1:?usage: each-component.sh <action> <lang> [component ...]}"
_lang="${2:?usage: each-component.sh <action> <lang> [component ...]}"
shift 2
_wanted=" $* "

if ! _hook="$(python3 "${HECATRON_HOME}/libexec/lib/project.py" resolve "${_lang}" "${_action}.sh")"; then
    echo "error: ${_lang} has no ${_action} hook (.hecatron/languages/${_lang}/${_action}.sh)" >&2
    exit 1
fi

# Lint checks formatting too, unless the project turned that off ("lint":
# {"format": false}) or the lint hook checked it itself (checks_format in
# languages/_hooks.sh, which writes to HECATRON_FORMAT_CHECKED).
_format_hook=""
if [ "${_action}" = lint ] && _in_list "${_lang}" ${_LINT_FORMAT_LANGS}; then
    _format_hook="$(python3 "${HECATRON_HOME}/libexec/lib/project.py" resolve "${_lang}" format.sh)"
fi
_format_checked="$(mktemp)"
# A hook with nothing to do in a component (audit without a lock file it
# reads) writes why here (skip_component in languages/_hooks.sh): skipped,
# not passed.
_skip_file="$(mktemp)"
trap 'rm -f "${_format_checked}" "${_skip_file}"' EXIT

_ran=0
_failed=()
_skipped=()
while IFS=$'\t' read -r _component _dir; do
    if [ "${_wanted}" != "  " ] && [[ "${_wanted}" != *" ${_component} "* ]]; then
        continue
    fi
    _ran=$((_ran + 1))
    echo "=== ${_action} ${_component} (${_lang}) ==="
    # The language's environment for this component's version (config.json
    # "path" and "env", docs/guide/configuration.md, docs/guide/versions.md), for its hooks only.
    if ! _env="$(python3 "${HECATRON_HOME}/libexec/lib/project.py" env "${_lang}" "${_component}" </dev/null)"; then
        _failed+=("${_component}")
        continue
    fi
    # stdin is the component list; a hook reading it (uv does) would swallow
    # the components after its own.
    : >"${_format_checked}"
    : >"${_skip_file}"
    _status=0
    (eval "${_env}" && cd "${_dir}" && HECATRON_COMPONENT="${_component}" HECATRON_LANG="${_lang}" \
        HECATRON_FORMAT_CHECKED="${_format_checked}" HECATRON_SKIPPED="${_skip_file}" \
        bash "${_hook}" "${_component}" </dev/null) || _status=1
    if [ -n "${_format_hook}" ] && [ ! -s "${_format_checked}" ]; then
        (eval "${_env}" && cd "${_dir}" && HECATRON_COMPONENT="${_component}" HECATRON_LANG="${_lang}" \
            HECATRON_FORMAT_CHECK=1 bash "${_format_hook}" "${_component}" </dev/null) || _status=1
    fi
    if [ "${_status}" -ne 0 ]; then
        _failed+=("${_component}")
    elif _reason="$(cat "${_skip_file}")" && [ -n "${_reason}" ]; then
        _skipped+=("${_component} (${_reason})")
    fi
done < <(python3 "${HECATRON_HOME}/libexec/lib/project.py" impl-dirs "${_lang}")

if [ "${_ran}" -eq 0 ]; then
    echo "error: no ${_lang} implementation found (implPath, or implPaths.${_lang}, in .hecatron/config.json)" >&2
    exit 1
fi
if [ "${#_failed[@]}" -gt 0 ]; then
    echo "${_lang}: ${_action} failed: ${_failed[*]}" >&2
    exit 1
fi
if [ "${#_skipped[@]}" -gt 0 ]; then
    _skip_list=""
    for _entry in "${_skipped[@]}"; do _skip_list="${_skip_list:+${_skip_list}, }${_entry}"; done
    echo "${_lang}: ${_action} skipped for ${#_skipped[@]} component(s): ${_skip_list}"
fi
_passed=$((_ran - ${#_skipped[@]}))
if [ "${_passed}" -gt 0 ]; then
    echo "${_lang}: ${_action} passed for ${_passed} component(s)"
fi

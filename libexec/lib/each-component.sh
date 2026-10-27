#!/usr/bin/env bash
# Run one of a language's hooks (test, lint, build or clean) for every
# component, or the ones named.
#
# Usage: each-component.sh <action> <lang> [component ...]
#
# The hook is .hecatron/languages/<lang>/<action>.sh in the project, else
# hecatron's built-in default (docs/design.md §7, §9). It runs from inside the
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

_ran=0
_failed=()
while IFS=$'\t' read -r _component _dir; do
    if [ "${_wanted}" != "  " ] && [[ "${_wanted}" != *" ${_component} "* ]]; then
        continue
    fi
    _ran=$((_ran + 1))
    echo "=== ${_action} ${_component} (${_lang}) ==="
    # The language's environment for this component's version (config.json
    # "path" and "env", docs/design.md §7, §18), for its hooks only.
    if ! _env="$(python3 "${HECATRON_HOME}/libexec/lib/project.py" env "${_lang}" "${_component}" </dev/null)"; then
        _failed+=("${_component}")
        continue
    fi
    # stdin is the component list; a hook reading it (uv does) would swallow
    # the components after its own.
    if ! (eval "${_env}" && cd "${_dir}" && HECATRON_COMPONENT="${_component}" HECATRON_LANG="${_lang}" \
            bash "${_hook}" "${_component}" </dev/null); then
        _failed+=("${_component}")
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
echo "${_lang}: ${_action} passed for ${_ran} component(s)"

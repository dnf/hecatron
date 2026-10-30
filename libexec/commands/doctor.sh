#!/usr/bin/env bash
# Check that each language's toolchain works, and say how to repair it.
#
# Usage: hecatron doctor [--fix] [lang ...]   (default: every added, enabled language)
#
# A language is healthy when the build and lint dependencies named in its
# config.json run, and its doctor.sh (the project's, else the built-in one)
# passes. A doctor.sh prints one line per problem and exits non-zero when it
# finds any; a line starting "note: " is shown under the language but isn't a
# problem. With --fix, an unhealthy language's install.sh runs (installers
# repair the states their doctor.sh detects), and the language is checked again.
set -euo pipefail

# shellcheck source=../lib/helpers.sh
source "${HECATRON_HOME}/libexec/lib/helpers.sh"

_resolve() { python3 "${HECATRON_HOME}/libexec/lib/project.py" resolve "$1" "$2"; }

if [ "${1:-}" = "help" ] || [ "${1:-}" = "--help" ]; then
    printf '%s\n' \
        "Usage: hecatron doctor [--fix [--no-deps]] [--output FORMAT] [language ...]" \
        "" \
        "Check that each language's toolchain works: its build and lint tools run, and" \
        "known breakages are absent. With no languages, checks every added, enabled one." \
        "" \
        "Flags:" \
        "  --fix      Run each unhealthy language's installer, then check it again" \
        "  --no-deps  With --fix, repair toolchains only: don't install the project's dependencies" \
        "  --output   human (the default), agent, or json: each language's problems and notes"
    exit 0
fi

_fix=false
_langs=()
_output=""
for _arg in "$@"; do
    if [ "${_output}" = next ]; then
        _output="${_arg}"
    elif [ "${_arg}" = "--output" ]; then
        _output=next
    elif [ "${_arg#--output=}" != "${_arg}" ]; then
        _output="${_arg#--output=}"
    elif [ "${_arg}" = "--fix" ]; then
        _fix=true
    elif [ "${_arg}" = "--no-deps" ]; then
        export HECATRON_NO_DEPS=1
    else
        _langs+=("$(_check "${_arg}" doctor)")
    fi
done
case "${_output}" in
    ""|human|agent) ;;
    json)
        _hint=""
        [ -e "${HECATRON_HOME}/.git" ] && _hint="; in this checkout, run hct self build"
        echo "hecatron: doctor --output json needs hecatron's binary, which install.sh downloads with a release${_hint}" >&2
        exit 2 ;;
    next) echo "hecatron: --output needs a format: human, agent or json" >&2; exit 2 ;;
    *) echo "hecatron: unknown output format '${_output}' (human, agent or json)" >&2; exit 2 ;;
esac
if [ "${#_langs[@]}" -eq 0 ]; then
    for _lang in ${_ADDED_LANGS}; do
        _is_disabled "${_lang}" || _langs+=("${_lang}")
    done
fi
if [ "${#_langs[@]}" -eq 0 ]; then
    echo "No languages to check (hecatron add <lang> adds one)."
    exit 0
fi

# Problems with one language, one per line; nothing when it is healthy.
_problems() (
    local _lang="$1" _dep _lint _script
    # In a subshell: the language's environment (its version's PATH entries
    # and variables), worked out now so a toolchain the installer just added
    # counts.
    eval "$(python3 "${HECATRON_HOME}/libexec/lib/project.py" env "${_lang}")" || true
    _dep="$(_dep "${_lang}")"
    _lint="$(_lint_dep "${_lang}")"
    if [ -n "${_dep}" ] && ! command -v "${_dep}" >/dev/null 2>&1; then
        echo "${_dep} not found"
        return
    fi
    # A note, not a problem: the toolchain works without its linter, and a
    # project can lint its own way (#254); hct lint still needs it.
    if [ -n "${_lint}" ] && ! command -v "${_lint}" >/dev/null 2>&1; then
        echo "note: lint tool ${_lint} not found: hct lint ${_lang} needs it (hct install ${_lang} installs it)"
    fi
    # Each component's version (docs/guide/versions.md), where the definition
    # can list what's installed.
    python3 "${HECATRON_HOME}/libexec/lib/project.py" missing-versions "${_lang}" |
        while IFS=$'\t' read -r _component _spec _source; do
            echo "${_component}: ${_lang} ${_spec} not installed (from ${_source})"
        done
    if _script="$(_resolve "${_lang}" doctor.sh)"; then
        (cd "${_ROOT}" && bash "${_script}") || true
    fi
)

# A doctor.sh line starting "note: " is worth saying (which toolchain a
# component pairs with which) but isn't a problem.
_notes_of() { printf '%s\n' "$1" | sed -n 's/^note: //p'; }
_problems_of() { printf '%s\n' "$1" | grep -v '^note: ' || true; }

_unhealthy=()
for _lang in "${_langs[@]}"; do
    _output="$(_problems "${_lang}")"
    _found="$(_problems_of "${_output}")"
    if [ -n "${_found}" ] && ${_fix}; then
        _installer="$(_resolve "${_lang}" install.sh)"
        echo "${_lang}: repairing with ${_installer}"
        bash "${_installer}" || true
        _output="$(_problems "${_lang}")"
        _found="$(_problems_of "${_output}")"
    fi
    _notes="$(_notes_of "${_output}")"
    if [ -z "${_found}" ]; then
        printf '  ok    %s\n' "${_lang}"
    else
        printf '  FAIL  %s\n' "${_lang}"
        printf '%s\n' "${_found}" | sed 's/^/          /'
        _unhealthy+=("${_lang}")
    fi
    if [ -n "${_notes}" ]; then
        printf '%s\n' "${_notes}" | sed 's/^/          /'
    fi
    # A component whose version comes from what it supports or its commit's
    # date (docs/guide/versions.md), so nobody wonders where the version came from.
    python3 "${HECATRON_HOME}/libexec/lib/project.py" version-notes "${_lang}" |
        while IFS=$'\t' read -r _component _note; do
            printf '          %s: %s %s\n' "${_component}" "${_lang}" "${_note}"
        done
done

echo
if [ "${#_unhealthy[@]}" -eq 0 ]; then
    echo "All ${#_langs[@]} toolchains look healthy."
    exit 0
fi
echo "${#_unhealthy[@]} of ${#_langs[@]} need attention: ${_unhealthy[*]}"
${_fix} || echo "Repair with: hecatron doctor --fix ${_unhealthy[*]}   (runs each language's installer)"
exit 1

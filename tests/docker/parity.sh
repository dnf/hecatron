#!/usr/bin/env bash
# Parity of the two ways a language installs (docs/installers.md): for each
# language, a fresh container installs it with its scripts
# (HECATRON_INSTALLER=script) and another from its install.json, both with
# hecatron's binary built from this checkout; each passes
# tests/install/check.sh's checks (install, doctor from a login shell,
# rerun, smoke), and what the machine ends up with must be the same:
# installed packages and versions, where the language's commands are, and the
# doctor's report. A language with project files in tests/docker/parity/<lang>/
# is checked with them, so what installs only for some files is covered.
#
#   tests/docker/parity.sh ubuntu:24.04 raku racket
#
# Needs Go and Docker. Prints a "### parity" line per language; exits 1 if
# any differ or fail.
set -uo pipefail

if [ "$#" -lt 2 ]; then
    echo "usage: $0 <base-image> <lang>..." >&2
    exit 2
fi
_base="$1"
shift
_here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_checkout="$(cd "${_here}/../.." && pwd)"
_work="$(mktemp -d)"
trap 'rm -rf "${_work}"' EXIT

_arch="$(uname -m | sed 's/x86_64/amd64/; s/aarch64/arm64/')"
(cd "${_checkout}" && CGO_ENABLED=0 GOOS=linux GOARCH="${_arch}" go build -trimpath -o "${_work}/hecatron" ./cmd/hecatron) || exit 1

# A side's state: the lines between "### state" and "### state end".
_state() { sed -n '/^### state packages$/,/^### state end$/p' "$1"; }

_failed=0
for _lang in "$@"; do
    _seed=""
    [ -d "${_here}/parity/${_lang}" ] && _seed="/hecatron/tests/docker/parity/${_lang}"
    for _side in script data; do
        _installer=""
        [ "${_side}" = script ] && _installer=script
        HECATRON_TEST_BINARY="${_work}/hecatron" HECATRON_INSTALLER="${_installer}" HECATRON_CHECK_SEED="${_seed}" \
            "${_here}/run.sh" "${_base}" bash /hecatron/tests/install/parity-run.sh "${_lang}" \
            >"${_work}/${_lang}.${_side}.log" 2>&1
        echo "$?" >"${_work}/${_lang}.${_side}.status"
    done
    _script_result="$(grep '^### result' "${_work}/${_lang}.script.log")"
    _data_result="$(grep '^### result' "${_work}/${_lang}.data.log")"
    _differences="$(diff <(_state "${_work}/${_lang}.script.log") <(_state "${_work}/${_lang}.data.log"))"
    _from_data="$(grep -c 'install\.json' "${_work}/${_lang}.data.log")"
    if [ "$(cat "${_work}/${_lang}.script.status")" != 0 ] || [ "$(cat "${_work}/${_lang}.data.status")" != 0 ] ||
        [ -n "${_differences}" ] || [ "${_from_data}" -eq 0 ]; then
        _failed=1
        echo "### parity ${_lang} on ${_base}: DIFFERENT"
        echo "  script: ${_script_result}"
        echo "  data:   ${_data_result} (install.json mentioned ${_from_data} times)"
        [ -n "${_differences}" ] && echo "${_differences}" | sed 's/^/  /'
        for _side in script data; do
            echo "--- ${_side} log (last 40 lines)"
            tail -40 "${_work}/${_lang}.${_side}.log" | sed 's/^/  /'
        done
    else
        echo "### parity ${_lang} on ${_base}: same (${_data_result#'### result '})"
    fi
    if [ -n "${HECATRON_PARITY_LOGS:-}" ]; then
        mkdir -p "${HECATRON_PARITY_LOGS}"
        cp "${_work}/${_lang}".*.log "${HECATRON_PARITY_LOGS}/"
    fi
done
exit "${_failed}"

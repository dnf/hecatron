#!/usr/bin/env bash
# hecatron version <version>: run each language's version.sh hook with the new
# version, from the project root. Languages without a version.sh are skipped.
set -euo pipefail

# shellcheck source=../lib/helpers.sh
source "${HECATRON_HOME}/libexec/lib/helpers.sh"

_version="${1:-}"
if [ -z "${_version}" ] || [ "${_version}" = "help" ] || [ "${_version}" = "--help" ]; then
    echo "Usage: hecatron version <version>   (runs each language's version.sh)"
    [ -z "${_version}" ] && exit 2
    exit 0
fi

cd "${HECATRON_ROOT}"
_updated=0
for _lang in ${_ADDED_LANGS}; do
    if _hook="$(python3 "${HECATRON_HOME}/libexec/lib/project.py" resolve "${_lang}" version.sh)"; then
        echo "==> ${_lang}"
        bash "${_hook}" "${_version}"
        _updated=$((_updated + 1))
    fi
done
echo "Set version ${_version} in ${_updated} language(s)."

#!/usr/bin/env bash
# hecatron self-test: the checks a change to hecatron must pass, zero
# ShellCheck warnings on every shell script and the unit tests, run on the
# hecatron installation itself (HECATRON_HOME) from anywhere.
set -euo pipefail

cd "${HECATRON_HOME}"
_failed=0

# Every shell script hecatron ships: *.sh, and extensionless scripts with a
# sh or bash shebang (bin/hecatron). examples/ is reference material.
_scripts() {
    local _file
    while IFS= read -r _file; do
        case "${_file}" in
            *.sh) echo "${_file}" ;;
            *.*) ;;
            *) if head -1 "${_file}" 2>/dev/null | grep -qE '^#!.*\b(ba)?sh\b'; then echo "${_file}"; fi ;;
        esac
    done < <(find bin libexec languages tests -type f 2>/dev/null | sort)
}

echo "=== shellcheck ==="
if ! command -v shellcheck >/dev/null 2>&1; then
    echo "shellcheck not found: brew install shellcheck, or sudo apt-get install shellcheck" >&2
    _failed=1
elif [ -d .git ] || [ -f .git ]; then
    # A checkout: the same check as CI, over every tracked script.
    bash libexec/lint/shellcheck.sh || _failed=1
else
    _list=()
    while IFS= read -r _script; do _list+=("${_script}"); done < <(_scripts)
    echo "shellcheck: ${#_list[@]} scripts"
    shellcheck -x -S warning "${_list[@]}" || _failed=1
fi

echo "=== unit tests ==="
if [ ! -d tests ]; then
    echo "no tests/ in ${HECATRON_HOME}: self-test needs a hecatron checkout (git clone https://github.com/dnf/hecatron)" >&2
    _failed=1
elif ! python3 -m unittest discover -s tests; then
    _failed=1
fi

if [ "${_failed}" -eq 0 ]; then
    echo "self-test passed"
else
    echo "self-test failed" >&2
fi
exit "${_failed}"

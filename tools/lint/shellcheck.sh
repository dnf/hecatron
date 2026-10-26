#!/usr/bin/env bash
# Run shellcheck (warnings and errors) on every tracked shell script: *.sh
# files and extensionless scripts with a sh or bash shebang. Scripts in other
# languages (the Lua, Perl, PHP... CLI launchers) are skipped.
set -euo pipefail

cd "$(dirname "$0")/../../.."

if ! command -v shellcheck >/dev/null 2>&1; then
    echo "error: shellcheck not found (brew install shellcheck, or apt-get install shellcheck)" >&2
    exit 1
fi

_is_shell_script() {
    case "$1" in
        *.sh) return 0 ;;
        *.*) return 1 ;;
    esac
    head -1 "$1" 2>/dev/null | grep -qE '^#!.*\b(ba)?sh\b'
}

_scripts=()
while IFS= read -r _file; do
    [ -f "${_file}" ] && _is_shell_script "${_file}" && _scripts+=("${_file}")
done < <(git ls-files)

echo "shellcheck: ${#_scripts[@]} scripts"
shellcheck -x -S warning "${_scripts[@]}"

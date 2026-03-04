# Shared helpers for per-language installers (sourced, not executed).
# shellcheck shell=bash

set -euo pipefail

# Platform: "macos" or "linux"
case "$(uname -s)" in
    Darwin) OS="macos" ;;
    Linux)  OS="linux" ;;
    *)      echo "Unsupported platform: $(uname -s)" >&2; exit 1 ;;
esac

# sudo only when not root
SUDO=""
if [ "$(id -u)" -ne 0 ]; then
    SUDO="sudo"
fi

# Project root (two levels up from .hecatron/languages/)
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

log() { echo "==> $*"; }

has() { command -v "$1" &>/dev/null; }

brew_install() { brew install "$@" || true; }

# Portable in-place sed (macOS requires empty backup suffix)
_sed_i() {
    if [ "${OS}" = "macos" ]; then
        sed -i '' "$@"
    else
        sed -i "$@"
    fi
}

apt_install() {
    # Refresh package list if stale (>1 hour) or missing
    local stamp="/var/lib/apt/lists/partial"
    if [ ! -d "/var/lib/apt/lists" ] || \
       [ -z "$(find /var/lib/apt/lists -maxdepth 1 -name '*.Release' 2>/dev/null | head -1)" ] || \
       [ -n "$(find "$stamp" -maxdepth 0 -mmin +60 2>/dev/null)" ]; then
        $SUDO apt-get update -qq
    fi
    $SUDO apt-get install -y "$@"
}

# Update metadata in a Gradle KTS build file (shared by Java, Kotlin, Scala).
# Reads PT_OLD_* and PT_* env vars set by .hecatron/tools/metadata/set.sh.
_set_metadata_gradle_kts() {
    local file="${1:?gradle file required}"

    # Top-level description
    _sed_i 's/^description = "'"${PT_OLD_DESCRIPTION}"'"/description = "'"${PT_DESCRIPTION}"'"/' "${file}"

    # POM description.set(...)
    _sed_i 's/description\.set("'"${PT_OLD_DESCRIPTION}"'")/description.set("'"${PT_DESCRIPTION}"'")/' "${file}"

    # POM url.set(...) — matches pom url + scm url (not license url which is a different value)
    _sed_i 's|url\.set("'"${PT_OLD_REPOSITORY}"'")|url.set("'"${PT_REPOSITORY}"'")|g' "${file}"

    # Developer name.set(...)
    _sed_i 's/name\.set("'"${PT_OLD_AUTHOR}"'")/name.set("'"${PT_AUTHOR}"'")/' "${file}"

    # SCM connection
    local old_host old_path new_host new_path
    old_host="$(echo "${PT_OLD_REPOSITORY}" | sed 's|https://||;s|/.*||')"
    old_path="$(echo "${PT_OLD_REPOSITORY}" | sed 's|https://[^/]*/||')"
    new_host="$(echo "${PT_REPOSITORY}" | sed 's|https://||;s|/.*||')"
    new_path="$(echo "${PT_REPOSITORY}" | sed 's|https://[^/]*/||')"
    _sed_i 's|connection\.set("scm:git:git://'"${old_host}/${old_path}"'\.git")|connection.set("scm:git:git://'"${new_host}/${new_path}"'.git")|' "${file}"
    _sed_i 's|developerConnection\.set("scm:git:ssh://'"${old_host}/${old_path}"'\.git")|developerConnection.set("scm:git:ssh://'"${new_host}/${new_path}"'.git")|' "${file}"
}

# Update metadata in a Groovy Gradle build file (Groovy DSL uses single quotes and = instead of .set()).
_set_metadata_gradle_groovy() {
    local file="${1:?gradle file required}"

    # Top-level description
    _sed_i "s/^description = '${PT_OLD_DESCRIPTION}'/description = '${PT_DESCRIPTION}'/" "${file}"

    # POM description
    _sed_i "s/description = '${PT_OLD_DESCRIPTION}'/description = '${PT_DESCRIPTION}'/" "${file}"

    # POM url (matches pom url + scm url, not license url)
    _sed_i "s|url = '${PT_OLD_REPOSITORY}'|url = '${PT_REPOSITORY}'|g" "${file}"

    # Developer name
    _sed_i "s/name = '${PT_OLD_AUTHOR}'/name = '${PT_AUTHOR}'/" "${file}"

    # SCM connection
    local old_host old_path new_host new_path
    old_host="$(echo "${PT_OLD_REPOSITORY}" | sed 's|https://||;s|/.*||')"
    old_path="$(echo "${PT_OLD_REPOSITORY}" | sed 's|https://[^/]*/||')"
    new_host="$(echo "${PT_REPOSITORY}" | sed 's|https://||;s|/.*||')"
    new_path="$(echo "${PT_REPOSITORY}" | sed 's|https://[^/]*/||')"
    _sed_i "s|connection = 'scm:git:git://${old_host}/${old_path}.git'|connection = 'scm:git:git://${new_host}/${new_path}.git'|" "${file}"
    _sed_i "s|developerConnection = 'scm:git:ssh://${old_host}/${old_path}.git'|developerConnection = 'scm:git:ssh://${new_host}/${new_path}.git'|" "${file}"
}

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

# The project being worked on (bin/hecatron exports HECATRON_ROOT), else the
# repository the current directory is in.
ROOT="${HECATRON_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"

log() { echo "==> $*"; }

# Each component's directory for a language, one per line: the project's
# HECATRON_IMPL_PATH template (e.g. {component}/impl/{lang}) filled in for each
# of HECATRON_COMPONENTS.
impl_dirs() {
    local _component _relative
    for _component in ${HECATRON_COMPONENTS:-.}; do
        _relative="${HECATRON_IMPL_PATH:-{lang\}}"
        _relative="${_relative//\{component\}/${_component}}"
        _relative="${_relative//\{lang\}/$1}"
        [ -d "${ROOT}/${_relative}" ] && (cd "${ROOT}/${_relative}" && pwd)
    done
    return 0
}

# Run a command inside each library's implementation of a language,
# e.g. `each_impl ruby bundle install`.
each_impl() {
    local _lang="$1" _dir
    shift
    while IFS= read -r _dir; do
        log "${_dir#"${ROOT}/"}: $*"
        (cd "${_dir}" && "$@")
    done < <(impl_dirs "${_lang}")
}

# For doctor.sh scripts: `problem` reports one problem, and `doctor_done`
# exits non-zero when any were reported.
_doctor_failed=0
problem() { echo "$*"; _doctor_failed=1; }
doctor_done() { exit "${_doctor_failed}"; }

has() { command -v "$1" &>/dev/null; }

brew_install() { brew install "$@" || true; }

# Homebrew sometimes installs a formula without linking it (e.g. when a stray
# file already sits in its prefix), leaving its commands off PATH. Link it when
# the command it provides is missing.
brew_ensure_linked() {
    local _formula="$1" _command="$2"
    has "${_command}" && return
    brew list --versions "${_formula}" >/dev/null 2>&1 || return 0
    log "Linking ${_formula} (${_command} is not on PATH)"
    brew link --overwrite "${_formula}"
}

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

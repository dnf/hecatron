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

# Each component's directory for a language, one per line, as hecatron
# resolves it (a language's own implPath and directory names); outside a
# hecatron run, the HECATRON_IMPL_PATH template (e.g. {component}/impl/{lang})
# filled in for each of HECATRON_COMPONENTS. Nothing outside a project.
impl_dirs() {
    if [ -n "${HECATRON_HOME:-}" ]; then
        (cd "${ROOT}" && python3 "${HECATRON_HOME}/libexec/lib/project.py" impl-dirs "$1" </dev/null 2>/dev/null) | cut -f2
        return 0
    fi
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

# Whether any of a language's components has a file a find expression
# matches, e.g. `impl_has bash -name '*.bats'`: for installing a tool only
# where a component needs it.
impl_has() {
    local _lang="$1" _dir
    shift
    while IFS= read -r _dir; do
        [ -n "$(find "${_dir}" \( "$@" \) -print -quit 2>/dev/null)" ] && return 0
    done < <(impl_dirs "${_lang}")
    return 1
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

# Refresh the package lists when they are missing or more than an hour old.
apt_refresh() {
    local stamp="/var/lib/apt/lists/partial"
    if [ ! -d "/var/lib/apt/lists" ] || \
       [ -z "$(find /var/lib/apt/lists -maxdepth 1 -name '*.Release' 2>/dev/null | head -1)" ] || \
       [ -n "$(find "$stamp" -maxdepth 0 -mmin +60 2>/dev/null)" ]; then
        $SUDO apt-get update -qq
    fi
}
apt_install() {
    apt_refresh
    $SUDO apt-get install -y "$@"
}
# Whether the configured apt sources offer a package: distributions drop and
# add toolchains between releases (Debian 13 has no OpenJDK 17).
# (Not piped into grep -q: under pipefail, apt-cache dying of SIGPIPE when grep
# exits early would fail the check.)
apt_has() {
    local policy
    apt_refresh
    policy="$(apt-cache policy "$1" 2>/dev/null)"
    [[ "${policy}" == *"Candidate: "[!\(]* ]]
}

# For Ruby's installer and doctor: whether Bundler would install into a gem
# directory this user can't write (apt's Ruby) with no path configured.
bundle_needs_user_path() {
    has ruby && has bundle || return 1
    [ ! -w "$(ruby -e 'print Gem.default_dir')" ] &&
        [ -z "$(bundle config get --parseable path 2>/dev/null)" ]
}

# For Perl's installer and doctor: the tools its components need and lack,
# one per line (make for Makefile.PL, Module::Build for Build.PL, and
# Perl::Critic where .perlcriticrc configures it).
perl_tools_missing() {
    local _dir _make=0 _build=0 _critic=0
    while IFS= read -r _dir; do
        [ -f "${_dir}/Makefile.PL" ] && _make=1
        [ -f "${_dir}/Build.PL" ] && _build=1
        [ -f "${_dir}/.perlcriticrc" ] && _critic=1
    done < <(impl_dirs perl)
    if [ "${_make}" -eq 1 ] && ! has make; then echo make; fi
    if [ "${_build}" -eq 1 ] && ! perl -MModule::Build -e 1 2>/dev/null; then echo Module::Build; fi
    if [ "${_critic}" -eq 1 ] && ! has perlcritic; then echo Perl::Critic; fi
    return 0
}

# For Elm's installer and doctor: whether a component configures elm-review
# (review/elm.json) and it isn't installed.
elm_review_missing() {
    local _dir
    has elm-review && return 1
    while IFS= read -r _dir; do
        [ -f "${_dir}/review/elm.json" ] && return 0
    done < <(impl_dirs elm)
    return 1
}

# For the native languages' installers (C, C++, Objective-C): the libraries a
# component's CMake build looks for with find_package(), as distribution
# packages, for the common ones. A name not in the table is left to the
# project (its README, or a vcpkg or Conan manifest).
_cmake_package() {
    local _name="$1"
    if [ "${OS}" = "macos" ]; then
        case "${_name}" in
            GTest | GMock) echo googletest ;;
            yaml-cpp) echo yaml-cpp ;;
            nlohmann_json) echo nlohmann-json ;;
            Boost) echo boost ;;
            fmt) echo fmt ;;
            spdlog) echo spdlog ;;
            Catch2) echo catch2 ;;
            benchmark) echo google-benchmark ;;
            Protobuf | protobuf) echo protobuf ;;
            OpenSSL) echo openssl ;;
            CURL) echo curl ;;
            PkgConfig) echo pkgconf ;;
        esac
    else
        case "${_name}" in
            GTest | GMock) echo "libgtest-dev libgmock-dev" ;;
            yaml-cpp) echo libyaml-cpp-dev ;;
            nlohmann_json) echo nlohmann-json3-dev ;;
            Boost) echo libboost-all-dev ;;
            fmt) echo libfmt-dev ;;
            spdlog) echo libspdlog-dev ;;
            Catch2) echo catch2 ;;
            benchmark) echo libbenchmark-dev ;;
            Protobuf | protobuf) echo "libprotobuf-dev protobuf-compiler" ;;
            OpenSSL) echo libssl-dev ;;
            CURL) echo libcurl4-openssl-dev ;;
            ZLIB) echo zlib1g-dev ;;
            PkgConfig) echo pkg-config ;;
            Threads) ;;
        esac
    fi
}
# The find_package() names in a language's components' CMake files.
cmake_find_packages() {
    local _dir
    while IFS= read -r _dir; do
        find "${_dir}" -maxdepth 3 \( -name build -o -name 'build-*' -o -name third_party -o -name external \) -prune -o \
            \( -name CMakeLists.txt -o -name '*.cmake' \) -type f -print 2>/dev/null
    done < <(impl_dirs "$1") |
        xargs -r sed -n 's/.*find_package[[:space:]]*([[:space:]]*\([A-Za-z0-9_+-]*\).*/\1/p' 2>/dev/null | sort -u
}
cmake_dependencies() {
    local _name _packages=""
    while IFS= read -r _name; do
        [ -n "${_name}" ] && _packages="${_packages} $(_cmake_package "${_name}")"
    done < <(cmake_find_packages "$1")
    _packages="$(echo "${_packages}" | xargs -n1 2>/dev/null | sort -u | xargs 2>/dev/null)"
    [ -n "${_packages}" ] || return 0
    log "Installing the libraries the CMake builds look for: ${_packages}"
    # shellcheck disable=SC2086 # one word per package
    if [ "${OS}" = "macos" ]; then brew_install ${_packages}; else apt_install ${_packages}; fi
}

# For the JVM languages' installers: Maven where a component builds with a
# pom.xml and ships no ./mvnw, Gradle where it builds with build.gradle(.kts)
# and ships no ./gradlew (the default hooks run the wrapper when there is one),
# and sbt where it builds with build.sbt (sbt's launcher fetches the sbt
# version project/build.properties names).
_JVM_GRADLE_VERSION="9.8.2"
_JVM_MAVEN_VERSION="3.9.16"
_JVM_SBT_VERSION="1.11.7"
# The build tools a language's components need and lack, one per line.
jvm_build_tools_missing() {
    local _dir _maven=0 _gradle=0 _sbt=0
    while IFS= read -r _dir; do
        { [ -f "${_dir}/pom.xml" ] && [ ! -x "${_dir}/mvnw" ]; } && _maven=1
        { { [ -f "${_dir}/build.gradle" ] || [ -f "${_dir}/build.gradle.kts" ]; } && [ ! -x "${_dir}/gradlew" ]; } && _gradle=1
        [ -f "${_dir}/build.sbt" ] && _sbt=1
    done < <(impl_dirs "$1")
    if [ "${_maven}" -eq 1 ] && ! _maven_current; then echo maven; fi
    if [ "${_gradle}" -eq 1 ] && ! has gradle; then echo gradle; fi
    if [ "${_sbt}" -eq 1 ] && ! has sbt; then echo sbt; fi
    return 0
}
jvm_build_tools() {
    local _missing
    _missing="$(jvm_build_tools_missing "$1")"
    if [[ "${_missing}" == *maven* ]]; then
        if [ "${OS}" = "macos" ]; then brew_install maven; else _install_maven; fi
    fi
    if [[ "${_missing}" == *gradle* ]]; then
        if [ "${OS}" = "macos" ]; then brew_install gradle; else _install_gradle; fi
    fi
    if [[ "${_missing}" == *sbt* ]]; then
        if [ "${OS}" = "macos" ]; then brew_install sbt; else _install_sbt; fi
    fi
}
# A Maven of 3.9 or later on PATH.
_maven_current() {
    local _version
    _version="$(mvn --version 2>/dev/null | sed -n '1s/^Apache Maven \([0-9]*\)\.\([0-9]*\).*/\1 \2/p')"
    [ -n "${_version}" ] && { set -- ${_version}; [ "$1" -gt 3 ] || { [ "$1" -eq 3 ] && [ "$2" -ge 9 ]; }; }
}
# Maven's own distribution: Ubuntu 24.04's Maven is 3.8, which projects that
# require 3.9 (maven-enforcer's requireMavenVersion) refuse.
_install_maven() {
    local _root="/opt/maven/apache-maven-${_JVM_MAVEN_VERSION}"
    if [ ! -x "${_root}/bin/mvn" ]; then
        log "Downloading Maven ${_JVM_MAVEN_VERSION}..."
        $SUDO mkdir -p /opt/maven
        curl -fsSL "https://repo.maven.apache.org/maven2/org/apache/maven/apache-maven/${_JVM_MAVEN_VERSION}/apache-maven-${_JVM_MAVEN_VERSION}-bin.tar.gz" |
            $SUDO tar -C /opt/maven -xz
    fi
    $SUDO ln -sfn "${_root}/bin/mvn" /usr/local/bin/mvn
}
# Gradle's own distribution: distributions' Gradle is years old.
_install_gradle() {
    local _root="/opt/gradle/gradle-${_JVM_GRADLE_VERSION}" _zip
    if [ ! -x "${_root}/bin/gradle" ]; then
        has unzip || apt_install unzip
        log "Downloading Gradle ${_JVM_GRADLE_VERSION}..."
        _zip="$(mktemp)"
        curl -fsSL -o "${_zip}" "https://services.gradle.org/distributions/gradle-${_JVM_GRADLE_VERSION}-bin.zip"
        $SUDO mkdir -p /opt/gradle
        $SUDO unzip -q -o "${_zip}" -d /opt/gradle
        rm -f "${_zip}"
    fi
    $SUDO ln -sfn "${_root}/bin/gradle" /usr/local/bin/gradle
}
# sbt's own launcher: Ubuntu and Debian don't package sbt.
_install_sbt() {
    local _root="/opt/sbt/sbt-${_JVM_SBT_VERSION}" _tgz
    if [ ! -x "${_root}/bin/sbt" ]; then
        log "Downloading sbt ${_JVM_SBT_VERSION}..."
        _tgz="$(mktemp)"
        curl -fsSL -o "${_tgz}" "https://github.com/sbt/sbt/releases/download/v${_JVM_SBT_VERSION}/sbt-${_JVM_SBT_VERSION}.tgz"
        $SUDO mkdir -p "${_root}"
        $SUDO tar xzf "${_tgz}" -C "${_root}" --strip-components=1
        rm -f "${_tgz}"
    fi
    $SUDO ln -sfn "${_root}/bin/sbt" /usr/local/bin/sbt
}

# For a definition's releases.sh: the latest release of each of a product's
# release lines from endoflife.date, newest first, as "version<TAB>flags",
# where flags lists "lts" (a long-term support line as of today) and "eol"
# (past its end of life), comma-separated. Build suffixes ("+11") are dropped.
endoflife_releases() {
    curl -fsSL "https://endoflife.date/api/$1.json" | python3 -c '
import datetime, json, sys
today = datetime.date.today().isoformat()
def reached(value):
    return value is True or (isinstance(value, str) and value <= today)
for cycle in json.load(sys.stdin):
    flags = [name for name, key in (("lts", "lts"), ("eol", "eol")) if reached(cycle.get(key))]
    print(str(cycle["latest"]).split("+")[0] + "\t" + ",".join(flags))
'
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

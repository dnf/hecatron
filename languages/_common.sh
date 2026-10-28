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

# Every download retries. Installing dozens of toolchains from dozens of hosts
# makes a transient failure (a timeout, a refused or dropped connection, a 5xx
# or 429) a matter of time, so curl tries up to five times, gives up on a
# connection after 30 s rather than curl's default of minutes, and treats a
# transfer stalled below 1 KB/s for a minute as a timeout, which it retries.
# A 404 isn't retried: it's an answer.
curl() {
    command curl --retry 4 --retry-delay 3 --retry-connrefused --connect-timeout 30 \
        --speed-limit 1024 --speed-time 60 "$@"
}

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
# each_impl with each component's own toolchain version (its "path" and "env",
# docs/design.md §18), for dependencies built for one version: a bundle's
# native extensions, a Composer lock resolved for the running PHP.
each_impl_versioned() {
    local _lang="$1" _component _dir
    shift
    if [ -z "${HECATRON_HOME:-}" ]; then
        each_impl "${_lang}" "$@"
        return
    fi
    while IFS=$'\t' read -r _component _dir; do
        log "${_dir#"${ROOT}/"}: $*"
        (eval "$(cd "${ROOT}" && python3 "${HECATRON_HOME}/libexec/lib/project.py" env "${_lang}" "${_component}" </dev/null)" &&
            cd "${_dir}" && "$@")
    done < <(cd "${ROOT}" && python3 "${HECATRON_HOME}/libexec/lib/project.py" impl-dirs "${_lang}" </dev/null 2>/dev/null)
}

# Each component's dependencies, through the language's deps hook (hecatron
# deps): the same code, frozen to each lock file, that the hooks use
# (languages/_deps.sh), and a project's own deps.sh where it has one.
# Only for a language the project has (TypeScript's installer also runs for
# JavaScript's Node.js) with a deps hook. While `hecatron install` installs
# every language, the language is only listed in the HECATRON_DEFER_DEPS
# file: the dependencies are fetched once every toolchain is in.
impl_deps() {
    [ -n "${HECATRON_HOME:-}" ] || return 0
    (cd "${ROOT}" && eval "$(python3 "${HECATRON_HOME}/libexec/lib/project.py" shell-env </dev/null)" &&
        printf '%s\n' "${_DEPS_LANGS}") | grep -qx "$1" || return 0
    if [ -n "${HECATRON_DEFER_DEPS:-}" ]; then
        echo "$1" >>"${HECATRON_DEFER_DEPS}"
        return 0
    fi
    (cd "${ROOT}" && bash "${HECATRON_HOME}/bin/hecatron" deps "$1" </dev/null)
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
# Something worth saying that isn't a problem (which toolchain a component
# pairs with which): shown under the language, without failing it.
note() { echo "note: $*"; }
doctor_done() { exit "${_doctor_failed}"; }

has() { command -v "$1" &>/dev/null; }

# For uninstall.sh: a tool another language's installer also installs (CMake,
# Node.js, the .NET SDK, a JDK...) stays when one of them is uninstalled
# (docs/installers.md, "Shared toolchains"), and the uninstaller says so.
left_shared() { echo "Left $1 installed: $2"; }
leave_cmake() {
    if has cmake; then left_shared CMake "C, C++, Objective-C, COBOL and Fortran share it."; fi
}
leave_node() {
    if has node; then left_shared Node.js "TypeScript, JavaScript and Elm share it."; fi
}
leave_dotnet() {
    if has dotnet || [ -d /opt/dotnet ] || [ -d "${HOME}/.local/share/hecatron/dotnet" ]; then
        left_shared "the .NET SDK" "C#, F# and VB.NET share it."
    fi
}

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
        $SUDO apt-get -o Acquire::Retries=3 update -qq
    fi
}
apt_install() {
    apt_refresh
    $SUDO apt-get -o Acquire::Retries=3 install -y "$@"
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

# Toolchain versions (docs/design.md §18). For an installer: each version the
# project's components resolve to that isn't installed yet, one spec per line
# ("1.22" or "=1.22.5"); nothing outside a hecatron run.
missing_versions() {
    [ -n "${HECATRON_HOME:-}" ] || return 0
    (cd "${ROOT}" && python3 "${HECATRON_HOME}/libexec/lib/project.py" missing-versions "$1" </dev/null) |
        cut -f2 | sort -u
}
# The newest of the versions on stdin, one per line, that a spec matches.
newest_match() {
    python3 -c '
import sys
sys.path.insert(0, sys.argv[2])
from versions import Spec, newest
spec = Spec(sys.argv[1])
print(newest([v.strip() for v in sys.stdin if spec.matches(v.strip())]) or "")
' "$1" "${HECATRON_HOME:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}/libexec/lib"
}
# A definition's default version (its config.json's "version", without "=").
default_version() {
    python3 -c 'import json, sys; print(json.load(open(sys.argv[1]))["version"].lstrip("="))' \
        "$(dirname "$0")/config.json"
}

# For Ruby's installer and doctor: whether Bundler would install into a gem
# directory this user can't write (apt's Ruby) with no path configured.
bundle_needs_user_path() {
    has ruby && has bundle || return 1
    [ ! -w "$(ruby -e 'print Gem.default_dir')" ] &&
        [ -z "$(bundle config get --parseable path 2>/dev/null)" ]
}

# perltidy's output changes between releases, so hecatron format runs one.
_PERL_TIDY_VERSION="20260826"
# For Perl's installer and doctor: the tools its components need and lack,
# one per line (make for Makefile.PL, Module::Build for Build.PL,
# Perl::Critic where .perlcriticrc configures it, and Perl::Tidy where
# .perltidyrc does), and the modules a Makefile.PL written with
# Module::Install needs before it can run: Module::Install itself and the
# configure_requires it names (its extensions, such as
# Module::Install::ReadmeFromPod), which a release bundles in inc/ but a
# checkout doesn't.
perl_tools_missing() {
    local _dir _make=0 _build=0 _critic=0 _tidy=0 _modules="" _module
    while IFS= read -r _dir; do
        [ -f "${_dir}/Makefile.PL" ] && _make=1
        [ -f "${_dir}/Build.PL" ] && _build=1
        [ -f "${_dir}/.perlcriticrc" ] && _critic=1
        [ -f "${_dir}/.perltidyrc" ] && _tidy=1
        grep -qs 'inc::Module::Install' "${_dir}/Makefile.PL" || continue
        _modules="${_modules} Module::Install $(sed -n \
            "s/^[[:space:]]*configure_requires[[:space:]]*(*[[:space:]]*['\"]\([A-Za-z0-9_:]*\)['\"].*/\1/p" "${_dir}/Makefile.PL")"
    done < <(impl_dirs perl)
    for _module in $(echo "${_modules}" | xargs -n1 2>/dev/null | sort -u); do
        perl_has_module "${_module}" || echo "${_module}"
    done
    if [ "${_make}" -eq 1 ] && ! has make; then echo make; fi
    if [ "${_build}" -eq 1 ] && ! perl_has_module Module::Build; then echo Module::Build; fi
    if [ "${_critic}" -eq 1 ] && ! has perlcritic; then echo Perl::Critic; fi
    if [ "${_tidy}" -eq 1 ] && [ "$(perltidy --version 2>/dev/null | sed -n 's/.*perltidy, v\([0-9]*\).*/\1/p')" != "${_PERL_TIDY_VERSION}" ]; then
        echo Perl::Tidy
    fi
    return 0
}
# (require, not -M: Module::Install's import dies outside a Makefile.PL.)
perl_has_module() { perl -e "require $1" 2>/dev/null; }

# For Dart's installer and doctor: whether a component is a Flutter package
# (its pubspec.yaml depends on the Flutter SDK, which `dart pub` refuses) and
# flutter isn't installed. The Dart hooks run flutter for these (dart_tool).
flutter_missing() {
    local _dir
    has flutter && return 1
    while IFS= read -r _dir; do
        is_flutter_package "${_dir}" && return 0
    done < <(impl_dirs dart)
    return 1
}
# (dependencies: flutter: sdk: flutter, or inline, flutter: { sdk: flutter })
is_flutter_package() { grep -qsE 'sdk:[[:space:]]*flutter([^[:alnum:]_]|$)' "$1/pubspec.yaml"; }

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

# For the C, C++ and Objective-C installers and doctors: whether a component
# formats with clang-format (a .clang-format) and neither uv, which runs the
# pinned clang-format (languages/_format.sh), nor a clang-format is installed.
clang_format_missing() {
    has uvx || [ -x "${HOME}/.local/bin/uvx" ] || has clang-format && return 1
    impl_configures "$1" .clang-format _clang-format
}

# For Elm's installer and doctor: elm-format, which hecatron format uses for
# every Elm component.
elm_format_missing() { ! has elm-format; }

# For Lua's installer and doctor: whether a component configures StyLua
# (stylua.toml) and hecatron's version of it isn't installed.
_STYLUA_VERSION="2.6.0"
stylua_missing() {
    [ "$(stylua --version 2>/dev/null)" != "stylua ${_STYLUA_VERSION}" ] && impl_configures lua stylua.toml .stylua.toml
}

# For R's installer and doctor: whether a component configures Air
# (air.toml) and hecatron's version of it isn't installed.
_AIR_VERSION="0.12.0"
air_missing() {
    [ "$(air --version 2>/dev/null)" != "air ${_AIR_VERSION}" ] && impl_configures r air.toml .air.toml
}

# For Julia's installer and doctor: whether a component configures
# JuliaFormatter (.JuliaFormatter.toml) and hecatron's version of it isn't in
# the shared environment the format hook uses (@hecatron-format).
_JULIA_FORMATTER_VERSION="2.14.0"
julia_formatter_missing() {
    impl_configures julia .JuliaFormatter.toml || return 1
    julia --project=@hecatron-format -e "import Pkg
exit(any(p -> p.name == \"JuliaFormatter\" && string(p.version) == \"${_JULIA_FORMATTER_VERSION}\", values(Pkg.dependencies())) ? 1 : 0)" \
        </dev/null >/dev/null 2>&1
}

# For the .NET languages' installers and doctors: the .NET runtimes the
# components' projects target (net8.0, net9.0...; not netstandard, which runs
# on any, nor the Windows-only .NET Framework's net4x), which their tests need.
dotnet_target_majors() {
    local _lang _dir
    for _lang in csharp fsharp vbnet; do impl_dirs "${_lang}"; done | while IFS= read -r _dir; do
        find "${_dir}" -maxdepth 4 \( -name bin -o -name obj \) -prune -o \
            \( -name '*.csproj' -o -name '*.fsproj' -o -name '*.vbproj' -o -name '*.props' \) -type f -print
    done |
        xargs -r grep -ho '<TargetFrameworks\?[^>]*>[^<]*' 2>/dev/null |
        sed 's/.*>//' | tr ';' '\n' | sed -n 's/^net\([0-9][0-9]*\)\.0.*$/\1/p' | sort -un
}
dotnet_has_runtime() { dotnet --list-runtimes 2>/dev/null | grep -q "^Microsoft.NETCore.App $1\."; }
dotnet_runtimes_missing() {
    local _major
    has dotnet || return 0
    while IFS= read -r _major; do
        dotnet_has_runtime "${_major}" || echo "${_major}"
    done < <(dotnet_target_majors)
}

# For the native languages' installers (C, C++, Objective-C): the libraries a
# component's CMake build looks for, as distribution packages, for the common
# ones. A name is a find_package() name, or a pkg-config module (from
# pkg_check_modules() or pkg_search_module()) as "pkg-config:<module>". A name
# not in the table is left to the project (its README, or a vcpkg or Conan
# manifest), except that on Debian and Ubuntu a pkg-config module <name> is
# looked for as lib<name>-dev, the usual package for it. Abseil is left out:
# projects built on it track its LTS releases and fetch the one they need when
# none is found, and a distribution's older one would be found first (and
# fail: protobuf links absl::absl_check, which Ubuntu 24.04's lacks).
_cmake_package() {
    local _name="${1#pkg-config:}"
    if [ "${OS}" = "macos" ]; then
        case "${_name}" in
            GTest | GMock | gtest | gmock | gtest_main) echo googletest ;;
            yaml-cpp) echo yaml-cpp ;;
            nlohmann_json) echo nlohmann-json ;;
            Boost) echo boost ;;
            fmt) echo fmt ;;
            spdlog) echo spdlog ;;
            Catch2) echo catch2 ;;
            doctest) echo doctest ;;
            benchmark) echo google-benchmark ;;
            Protobuf | protobuf) echo protobuf ;;
            re2) echo re2 ;;
            ICU | icu-uc | icu-i18n) echo icu4c ;;
            OpenSSL | openssl | libssl) echo openssl ;;
            CURL | libcurl) echo curl ;;
            PkgConfig) echo pkgconf ;;
            GLIB | glib-2.0 | gobject-2.0 | gio-2.0 | gio-unix-2.0) echo glib ;;
            jansson) echo jansson ;;
            Snappy | snappy) echo snappy ;;
            zstd | ZSTD | libzstd) echo zstd ;;
            lz4 | LZ4 | liblz4) echo lz4 ;;
            LibLZMA | liblzma) echo xz ;;
            Libevent | libevent) echo libevent ;;
            JsonCpp | jsoncpp) echo jsoncpp ;;
            libuv) echo libuv ;;
            FLEX) echo flex ;;
            BISON) echo bison ;;
        esac
    else
        case "${_name}" in
            GTest | GMock | gtest | gmock | gtest_main) echo "libgtest-dev libgmock-dev" ;;
            yaml-cpp) echo libyaml-cpp-dev ;;
            nlohmann_json) echo nlohmann-json3-dev ;;
            Boost) echo libboost-all-dev ;;
            fmt) echo libfmt-dev ;;
            spdlog) echo libspdlog-dev ;;
            Catch2) echo catch2 ;;
            doctest) echo doctest-dev ;;
            benchmark) echo libbenchmark-dev ;;
            Protobuf | protobuf) echo "libprotobuf-dev protobuf-compiler" ;;
            re2) echo libre2-dev ;;
            ICU | icu-uc | icu-i18n) echo libicu-dev ;;
            OpenSSL | openssl | libssl) echo libssl-dev ;;
            CURL | libcurl) echo libcurl4-openssl-dev ;;
            ZLIB | zlib) echo zlib1g-dev ;;
            PkgConfig) echo pkg-config ;;
            GLIB | glib-2.0 | gobject-2.0 | gio-2.0 | gio-unix-2.0) echo libglib2.0-dev ;;
            Snappy | snappy) echo libsnappy-dev ;;
            zstd | ZSTD) echo libzstd-dev ;;
            lz4 | LZ4) echo liblz4-dev ;;
            BZip2) echo libbz2-dev ;;
            Libevent) echo libevent-dev ;;
            JsonCpp | jsoncpp) echo libjsoncpp-dev ;;
            libuv) echo libuv1-dev ;;
            CyrusSASL) echo libsasl2-dev ;;
            FLEX) echo flex ;;
            BISON) echo bison ;;
            Threads) ;;
            liblzma | LibLZMA) echo liblzma-dev ;;
            lib*) if [ "$1" != "${_name}" ]; then echo "${_name}-dev"; fi ;;
            *) if [ "$1" != "${_name}" ]; then echo "lib${_name}-dev"; fi ;;
        esac
    fi
}
# The directory a component's CMake build configures from: its own, or, when
# its CMakeLists.txt calls no project() (a subdirectory that a parent's
# add_subdirectory() pulls in, which can't be configured alone), the nearest
# one above it in the project whose CMakeLists.txt does. The native hooks
# configure from the same directory (cmake_source_dir in _hooks.sh).
cmake_project_dir() {
    local _dir="$1"
    while [ -f "$1/CMakeLists.txt" ] && ! _cmake_has_project "${_dir}"; do
        if [ "${_dir}" = "${ROOT}" ] || [ "${_dir}" = / ]; then
            echo "$1"
            return 0
        fi
        _dir="$(dirname "${_dir}")"
    done
    echo "${_dir}"
}
_cmake_has_project() { grep -qiE '^[[:space:]]*project[[:space:]]*\(' "$1/CMakeLists.txt" 2>/dev/null; }
# What a language's components' CMake builds look for, one name per line (see
# _cmake_package): find_package() names, pkg-config modules, and the packages
# behind a few headers that builds look for by hand (find_path(... gtest/gtest.h)).
# Fetched or vendored dependencies and configured build trees aren't searched.
cmake_find_packages() {
    local _dir
    while IFS= read -r _dir; do
        echo "${_dir}"
        cmake_project_dir "${_dir}"
    done < <(impl_dirs "$1") | sort -u | while IFS= read -r _dir; do
        find "${_dir}" -maxdepth 3 \( -name _deps -o -name CMakeFiles -o -name third_party -o -name external \
            -o -type d -exec test -f '{}/CMakeCache.txt' \; \) -prune -o \
            \( -name CMakeLists.txt -o -name '*.cmake' \) -type f -print 2>/dev/null
    done | sort -u | python3 -c '
import os, re, sys
HEADERS = {r"gtest/gtest\.h": "GTest", r"gmock/gmock\.h": "GMock", r"re2/re2\.h": "re2",
           r"unicode/\w+\.h": "ICU", r"google/protobuf/\w+\.h": "Protobuf"}
names = set()
paths = sys.stdin.read().splitlines()
seen = set()
while paths:
    path = os.path.normpath(paths.pop())
    if path in seen:
        continue
    seen.add(path)
    try:
        text = re.sub(r"#[^\n]*", "", open(path, errors="replace").read())
    except OSError:
        continue
    # A file include()d by path, as from a shared tools directory outside the component.
    for included in re.findall(r"include\s*\(\s*\"?([^\s)\"]+\.cmake)", text, re.I):
        included = re.sub(r"\$\{CMAKE_CURRENT_(?:SOURCE|LIST)_DIR\}", os.path.dirname(path), included)
        if "$" not in included:
            paths.append(os.path.join(os.path.dirname(path), included))
    names.update(re.findall(r"find_package\s*\(\s*([A-Za-z0-9_+-]+)", text, re.I))
    # A find module (FindFoo.cmake) looks for what find_package(Foo) names.
    if re.fullmatch(r"Find.+\.cmake", os.path.basename(path)):
        continue
    for call in re.findall(r"pkg_(?:check|search)_modules?\s*\(\s*\S+\s+([^)]*)\)", text, re.I):
        for word in call.split():
            module = re.split(r"[<>=]", word)[0]
            if re.fullmatch(r"[A-Za-z0-9_.+-]+", module) and not module.isupper():
                names.add("pkg-config:" + module)
    names.update(name for header, name in HEADERS.items() if re.search(header, text))
print("\n".join(sorted(names)))
'
}
cmake_dependencies() {
    local _name _package _packages=""
    while IFS= read -r _name; do
        [ -n "${_name}" ] && _packages="${_packages} $(_cmake_package "${_name}")"
    done < <(cmake_find_packages "$1")
    _packages="$(echo "${_packages}" | xargs -n1 2>/dev/null | sort -u | xargs 2>/dev/null)"
    # Distributions name and split packages differently, and a lib<name>-dev
    # guess may name none: install the ones there are.
    if [ "${OS}" = "linux" ] && [ -n "${_packages}" ]; then
        local _available=""
        for _package in ${_packages}; do
            if apt_has "${_package}"; then _available="${_available} ${_package}"; fi
        done
        _packages="${_available# }"
    fi
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
        _gradle_project_above "${_dir}" && _gradle=1
    done < <(impl_dirs "$1")
    if [ "${_maven}" -eq 1 ] && ! _maven_current; then echo maven; fi
    if [ "${_gradle}" -eq 1 ] && ! has gradle; then echo gradle; fi
    if [ "${_sbt}" -eq 1 ] && ! has sbt; then echo sbt; fi
    return 0
}
# Whether a directory with no build file of its own is a project of a Gradle
# build above it, in the project, without a wrapper (the hooks run Gradle
# from the directory, and it finds the settings itself).
_gradle_project_above() {
    local _dir="$1" _root
    [ -f "${_dir}/pom.xml" ] || [ -f "${_dir}/build.gradle" ] || [ -f "${_dir}/build.gradle.kts" ] ||
        [ -f "${_dir}/build.sbt" ] && return 1
    _root="$(cd "${ROOT}" && pwd -P)"
    _dir="$(cd "${_dir}" && pwd -P)"
    while [ "${_dir}" != "${_root}" ] && [ "${_dir}" != / ]; do
        _dir="$(dirname "${_dir}")"
        if [ -f "${_dir}/settings.gradle" ] || [ -f "${_dir}/settings.gradle.kts" ]; then
            [ ! -x "${_dir}/gradlew" ]
            return
        fi
    done
    return 1
}
jvm_build_tools() {
    local _missing
    jvm_jdks "$1"
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
# The JDKs (feature versions) a language's components' builds need besides
# the one they run on, and that aren't installed, one per line: the ones
# their Gradle or Maven toolchains name, which Gradle and Maven find among the
# installed JDKs, and, for a build whose Gradle wrapper or Kotlin compiler
# predates the default JDK, the newest LTS JDK it runs on, which the hooks
# run it on (jvm_select_jdk in _hooks.sh). See libexec/lib/jvm.py.
jvm_jdks_missing() {
    [ -n "${HECATRON_HOME:-}" ] || return 0
    local _dir _installed _feature _ceiling _why
    _installed="$(bash "${HECATRON_HOME}/languages/java/installed.sh" 2>/dev/null | cut -f1 |
        sed 's/^1\.\([0-9]*\).*/\1/; s/[^0-9].*//' | sort -un)"
    while IFS= read -r _dir; do
        while IFS= read -r _feature; do
            grep -qx "${_feature}" <<<"${_installed}" || echo "${_feature}"
        done < <(python3 "${HECATRON_HOME}/libexec/lib/jvm.py" toolchains "${_dir}" "${ROOT}")
        IFS=$'\t' read -r _ceiling _why < <(python3 "${HECATRON_HOME}/libexec/lib/jvm.py" ceiling "${_dir}" "${ROOT}") || continue
        [ -n "$(awk -v ceiling="${_ceiling}" '$1 <= ceiling' <<<"${_installed}")" ] || echo "${_ceiling}"
    done < <(impl_dirs "$1") | sort -un
}
jvm_jdks() {
    local _jdks
    _jdks="$(jvm_jdks_missing "$1")"
    [ -n "${_jdks}" ] || return 0
    # shellcheck disable=SC2086 # one feature version per word
    bash "${HECATRON_HOME}/languages/java/install.sh" ${_jdks}
}
# A Maven of 3.9 or later on PATH.
_maven_current() {
    local _version
    _version="$(mvn --version 2>/dev/null | sed -n '1s/^Apache Maven \([0-9]*\)\.\([0-9]*\).*/\1 \2/p')"
    [ -n "${_version}" ] && { set -- ${_version}; [ "$1" -gt 3 ] || { [ "$1" -eq 3 ] && [ "$2" -ge 9 ]; }; }
}
# A JVM language's own compiler and runner (Groovy, Scala, Kotlin), from its
# official distribution archive (.zip or .tar.gz with one top directory) into
# /opt/<name>/<version>, with the named commands linked into /usr/local/bin.
#   jvm_language_install groovy 5.1.3 https://.../groovy-binary-5.1.3.zip groovy groovyc
jvm_language_install() {
    local _name="$1" _version="$2" _url="$3" _root _archive _tool
    shift 3
    _root="/opt/${_name}/${_version}"
    if [ ! -d "${_root}/bin" ]; then
        log "Downloading ${_name} ${_version}..."
        _archive="$(mktemp)"
        curl -fsSL -o "${_archive}" "${_url}"
        $SUDO mkdir -p "${_root}"
        case "${_url}" in
            *.zip)
                has unzip || apt_install unzip
                local _unpacked
                _unpacked="$(mktemp -d)"
                unzip -q "${_archive}" -d "${_unpacked}"
                $SUDO cp -R "${_unpacked}"/*/. "${_root}/"
                rm -rf "${_unpacked:?}" ;;
            *) $SUDO tar -C "${_root}" --strip-components=1 -xzf "${_archive}" ;;
        esac
        rm -f "${_archive}"
    fi
    for _tool in "$@"; do
        $SUDO ln -sfn "${_root}/bin/${_tool}" "/usr/local/bin/${_tool}"
    done
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

# Whether a component configures a tool with one of the files: in its
# directory, below it, or above it up to the project root (where formatters
# that look upward find their configuration), e.g.
# `impl_configures lua stylua.toml .stylua.toml`.
impl_configures() {
    local _lang="$1" _dir _name
    shift
    while IFS= read -r _dir; do
        for _name in "$@"; do
            [ -n "$(find "${_dir}" -maxdepth 3 -name "${_name}" -print -quit 2>/dev/null)" ] && return 0
        done
        while [ "${_dir}" != "${ROOT}" ] && [ "${_dir}" != / ]; do
            _dir="$(dirname "${_dir}")"
            for _name in "$@"; do
                [ -e "${_dir}/${_name}" ] && return 0
            done
        done
    done < <(impl_dirs "${_lang}")
    return 1
}

# uv (Astral's), from its installer into ~/.local/bin: Python's toolchain,
# and how hecatron runs pinned tools published on PyPI (clang-format).
uv_install() {
    [ -x "${HOME}/.local/bin/uv" ] || curl -LsSf https://astral.sh/uv/install.sh | sh
    # uv lands in ~/.local/bin, which this shell's PATH may predate.
    export PATH="${HOME}/.local/bin:${PATH}"
}

sha256_of() { if has sha256sum; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1; }

# A tool's release download, checked against its checksum, into
# /usr/local/bin: the binary itself, or the file at a path inside a .zip or
# .tar.gz. Usage: release_binary_install NAME URL SHA256 [PATH-IN-ARCHIVE]
release_binary_install() {
    local _name="$1" _url="$2" _sha256="$3" _member="${4:-$1}" _work
    _work="$(mktemp -d)"
    curl -fsSL -o "${_work}/download" "${_url}"
    if [ "$(sha256_of "${_work}/download")" != "${_sha256}" ]; then
        rm -rf "${_work}"
        echo "error: the ${_name} download doesn't match its checksum (${_url})" >&2
        return 1
    fi
    case "${_url}" in
        *.zip) (cd "${_work}" && python3 -m zipfile -e download .) ;;
        *.tar.gz | *.tgz) tar -C "${_work}" -xzf "${_work}/download" ;;
        *) mv "${_work}/download" "${_work}/${_member}" ;;
    esac
    $SUDO install -m 0755 "${_work}/${_member}" "/usr/local/bin/${_name}"
    rm -rf "${_work}"
}

# A language's default version (config.json's "version", without its "="),
# for an installer that downloads a release: the version the binary installs
# from install.json too.
default_version() {
    python3 -c 'import json, sys; print(json.load(open(sys.argv[1]))["version"].lstrip("="))' \
        "$(dirname "${BASH_SOURCE[0]}")/$1/config.json"
}

# A release download, checked against its SHA-256, unpacked into hecatron's
# toolchains directory as the binary's install.json installs it
# (~/.local/share/hecatron/toolchains/<lang>/<version>; no root), and its
# commands linked with toolchain_link. Unpacked in a temporary directory and
# moved into place, so an interrupted install leaves nothing half done. Usage:
#   toolchain_release_install LANG VERSION URL SHA256 STRIP
toolchain_release_install() {
    local _lang="$1" _version="$2" _url="$3" _sha256="$4" _strip="$5" _home _work _flag=-xzf
    _home="$(toolchain_home "${_lang}" "${_version}")"
    _work="$(mktemp -d)"
    curl -fsSL -o "${_work}/download" "${_url}"
    if [ "$(sha256_of "${_work}/download")" != "${_sha256}" ]; then
        rm -rf "${_work}"
        echo "error: the ${_lang} download doesn't match its checksum (${_url})" >&2
        return 1
    fi
    case "${_url}" in
        *.tar.xz) _flag=-xJf ;;
    esac
    mkdir -p "${_work}/home" "$(dirname "${_home}")"
    tar -C "${_work}/home" --strip-components "${_strip}" "${_flag}" "${_work}/download"
    rm -rf "${_home}"
    mv "${_work}/home" "${_home}"
    rm -rf "${_work}"
}

# hecatron's own data directory, and a version's directory in it.
hecatron_data() { echo "${XDG_DATA_HOME:-${HOME}/.local/share}/hecatron"; }
toolchain_home() { echo "$(hecatron_data)/toolchains/$1/$2"; }

# Link commands from a toolchain directory into ~/.local/share/hecatron/bin.
# Usage: toolchain_link HOME COMMAND...
toolchain_link() {
    local _home="$1" _command _bin
    shift
    _bin="$(hecatron_data)/bin"
    mkdir -p "${_bin}"
    for _command in "$@"; do
        ln -sfn "${_home}/${_command}" "${_bin}/${_command}"
    done
}

# Remove a language's toolchains directory and the links into it.
toolchain_uninstall() {
    local _dir _link
    _dir="$(hecatron_data)/toolchains/$1"
    for _link in "$(hecatron_data)"/bin/*; do
        case "$(readlink "${_link}" 2>/dev/null)" in
            "${_dir}"/*) rm -f "${_link}" ;;
        esac
    done
    rm -rf "${_dir}"
}

# For installed.sh: "version<TAB>home" for each of a language's versions in
# the toolchains directory whose COMMAND (relative to the home) is there.
toolchain_installed() {
    local _home
    for _home in "$(hecatron_data)/toolchains/$1"/*; do
        [ -x "${_home}/$2" ] && printf '%s\t%s\n' "${_home##*/}" "${_home}"
    done
    return 0
}

# For the install.sh of a language installed from its install.json
# (docs/installers.md), which other installers still run directly (Elixir's
# and Gleam's run Erlang's): install from the data through hecatron's binary,
# as `hecatron install` would, and exit with its status. The script carries
# on itself without the binary (HECATRON_BINARY, which the binary gives the
# installers it runs), under HECATRON_INSTALLER=script, or when the binary
# declines the language (status 125: a project's own install.sh, a platform
# the data doesn't cover). `hecatron _install` never runs a script, so the
# two can't call each other in a loop.
install_from_data() {
    local _status=0
    [ "${HECATRON_INSTALLER:-}" != script ] || return 0
    [ -n "${HECATRON_BINARY:-}" ] && [ -x "${HECATRON_BINARY}" ] || return 0
    "${HECATRON_BINARY}" _install "$1" || _status=$?
    [ "${_status}" -eq 125 ] && return 0
    exit "${_status}"
}

# OSV-Scanner (Google's), the dependency auditor `hecatron audit` uses for
# ecosystems without one of their own (docs/audit.md), and what `hecatron
# licenses` reads licenses with (docs/licenses.md): its release binary,
# pinned and checked against its checksum, into /usr/local/bin.
_OSV_SCANNER_VERSION="2.7.0"
osv_scanner_install() {
    local _platform _sha256 _binary
    if has osv-scanner && [ "$(osv-scanner --version 2>/dev/null | sed -n 's/^osv-scanner version: //p')" = "${_OSV_SCANNER_VERSION}" ]; then
        return 0
    fi
    case "${OS}-$(uname -m)" in
        linux-x86_64) _platform=linux_amd64 _sha256=160c6253e2922e9e0a6a9771c5c4c40a4b3c6090b34a3cdd5e80c96cc2e4218b ;;
        linux-aarch64 | linux-arm64) _platform=linux_arm64 _sha256=4061ef4c021ed3c47f203b818f337e351336cc5cd67c0274fc36bd1dfc728dd7 ;;
        macos-x86_64) _platform=darwin_amd64 _sha256=e82c5df7ab0a42dff29a1cd0660711447b56b4c0483ff66ad8634df10fd81a45 ;;
        macos-arm64) _platform=darwin_arm64 _sha256=d4926ef8da134332543be276dff157e4c170f99ad91fc48e89448a8816bdf70b ;;
        *) echo "WARNING: no OSV-Scanner release for $(uname -sm); hecatron audit and licenses can't use it" >&2; return 0 ;;
    esac
    log "Installing OSV-Scanner ${_OSV_SCANNER_VERSION} (for hecatron audit and licenses)..."
    _binary="$(mktemp)"
    curl -fsSL -o "${_binary}" \
        "https://github.com/google/osv-scanner/releases/download/v${_OSV_SCANNER_VERSION}/osv-scanner_${_platform}"
    if [ "$(sha256_of "${_binary}")" != "${_sha256}" ]; then
        rm -f "${_binary}"
        echo "error: the OSV-Scanner download doesn't match its checksum" >&2
        return 1
    fi
    $SUDO install -m 0755 "${_binary}" /usr/local/bin/osv-scanner
    rm -f "${_binary}"
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

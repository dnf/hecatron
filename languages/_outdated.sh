# Shared helpers for the default outdated hooks (languages/<lang>/hooks/outdated.sh;
# sourced after _hooks.sh). Each runs its ecosystem's own report of newer
# versions of the component's direct dependencies and hands it to
# libexec/lib/outdated.py, which prints package, current, wanted and latest
# (docs/guide/outdated.md). Report-only: a component fails only for an error, or
# with --fail (HECATRON_OUTDATED_FAIL=1) when anything is outdated. The
# tools ask their registries online.
# shellcheck shell=bash
# shellcheck disable=SC2154 # build_dir is set by find_build in _hooks.sh

set -euo pipefail

_outdated_home="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
_outdated_report_file="$(mktemp)"
_outdated_direct_file=""
_outdated_cleanup() {
    rm -f "${_outdated_report_file}"
    if [ -n "${_outdated_direct_file}" ]; then rm -f "${_outdated_direct_file}"; fi
}
trap _outdated_cleanup EXIT

# Run a tool, its report into the report file; its messages pass through.
# Most exit non-zero when something is outdated, so the status isn't read:
# a report outdated.py can't parse is the error.
outdated_run() {
    echo "+ $*"
    "$@" >"${_outdated_report_file}" || true
}

# libexec/lib/outdated.py's commands, run by hecatron's binary when the run
# commands give it (HECATRON_BINARY: the same reader, without Python).
outdated_reader() {
    if [ -n "${HECATRON_BINARY:-}" ]; then
        "${HECATRON_BINARY}" _outdated "$@"
    else
        python3 "${_outdated_home}/libexec/lib/outdated.py" "$@"
    fi
}

# The rows in the report file, in a format outdated.py reads (and the raw
# report, saved to HECATRON_OUTDATED_REPORT_DIR when set). Exits.
outdated_finish() {
    local _status=0
    if [ -n "${HECATRON_OUTDATED_REPORT_DIR:-}" ]; then
        mkdir -p "${HECATRON_OUTDATED_REPORT_DIR}"
        cp "${_outdated_report_file}" "${HECATRON_OUTDATED_REPORT_DIR}/${HECATRON_LANG:-}-${HECATRON_COMPONENT:-}-$1.txt"
    fi
    outdated_reader report "$1" "${_outdated_report_file}" \
        "${HECATRON_LANG:-}" "${HECATRON_COMPONENT:-.}" ${_outdated_direct_file:+"${_outdated_direct_file}"} ||
        _status=$?
    exit "${_status}"
}

# Nothing this tool can report on here: say so, and succeed.
outdated_nothing() {
    echo "${HECATRON_LANG:-}: outdated: nothing to check: $*"
    exit 0
}

# Node.js: the package manager's own outdated report, on the installed
# packages (installed first when they aren't). Yarn 2 and later and Bun have
# no machine-readable one, so npm's reads their node_modules; Yarn's Plug'n'Play
# has none to read.
outdated_node() {
    [ -f package.json ] || outdated_nothing "no package.json"
    # shellcheck source=languages/typescript/package-manager.sh
    source "${_outdated_home}/languages/typescript/package-manager.sh"
    node_pm_command
    npm_deps
    case "${node_pm_name}" in
        pnpm)
            outdated_run "${node_pm[@]}" outdated --format json
            outdated_finish pnpm ;;
        yarn)
            if [ "$("${node_pm[@]}" --version 2>/dev/null | cut -d. -f1)" = 1 ]; then
                outdated_run "${node_pm[@]}" outdated --json
                outdated_finish yarn-classic
            fi
            [ -d node_modules ] || outdated_nothing "Yarn's Plug'n'Play has no node_modules for npm outdated to read" ;;
    esac
    outdated_run npm outdated --json
    outdated_finish npm
}

# Python: uv's view of the project's direct dependencies against uv.lock;
# without a lock, every package in the component's .venv.
outdated_python() {
    has uv || { echo "error: python: uv isn't installed (hecatron install python installs it)" >&2; exit 1; }
    if [ -f pyproject.toml ] && nearest_with uv.lock >/dev/null; then
        outdated_run uv tree --outdated --depth 1 --frozen
        outdated_finish uv-tree
    fi
    [ -d .venv ] || outdated_nothing "no uv.lock and no .venv (hecatron deps python makes one)"
    outdated_run uv pip list --outdated --format json
    outdated_finish uv-pip
}

# Go: the module's direct requirements with a newer version.
outdated_go() {
    find_build --above-first go.mod || outdated_nothing "no go.mod"
    outdated_run go list -u -m -json all
    outdated_finish go
}

# Rust: Cargo's own dry-run update, which names the newest version of each
# locked package (available: ...), filtered to the direct dependencies
# (cargo metadata). No plugin needed.
outdated_rust() {
    find_build --above-first Cargo.toml || outdated_nothing "no Cargo.toml"
    _outdated_direct_file="$(mktemp)"
    cargo metadata --no-deps --format-version 1 >"${_outdated_direct_file}"
    echo "+ cargo update --dry-run --verbose"
    cargo update --dry-run --verbose >"${_outdated_report_file}" 2>&1 || true
    outdated_finish cargo
}

# Ruby: Bundler's report on the Gemfile's own gems.
outdated_ruby() {
    ruby_project
    [ -f "${ruby_root}/Gemfile" ] || outdated_nothing "no Gemfile"
    bundle_deps
    outdated_run bundle outdated --parseable --only-explicit
    outdated_finish bundler
}

# PHP: Composer's report on composer.json's own packages, from the lock.
outdated_php() {
    [ -f composer.json ] || outdated_nothing "no composer.json"
    if [ -f composer.lock ]; then
        outdated_run composer outdated --locked --direct --format=json --no-interaction
    else
        composer_deps
        outdated_run composer outdated --direct --format=json --no-interaction
    fi
    outdated_finish composer
}

# .NET: the SDK's report on each project's top-level packages, after a
# restore.
outdated_dotnet() {
    find_build '*.sln' '*.slnx' '*.csproj' '*.fsproj' '*.vbproj' || outdated_nothing "no .sln, .slnx or project file"
    note_build_dir
    cd "${build_dir}"
    local _target
    _target="$(dotnet_target)"
    run dotnet restore ${_target:+"${_target}"} >/dev/null
    outdated_run dotnet list ${_target:+"${_target}"} package --outdated --format json
    outdated_finish dotnet
}

# Elixir: Hex's report on mix.exs's dependencies.
outdated_elixir() {
    [ -f mix.exs ] || outdated_nothing "no mix.exs"
    mix_deps
    outdated_run mix hex.outdated
    outdated_finish mix
}

# Dart: pub's report, on the direct and dev dependencies.
outdated_dart() {
    [ -f pubspec.yaml ] || outdated_nothing "no pubspec.yaml"
    dart_deps
    outdated_run "$(dart_tool)" pub outdated --json
    outdated_finish dart
}

# Haskell: cabal's report on the .cabal file's bounds. Stack has none.
outdated_haskell() {
    exists '*.cabal' || outdated_nothing "no .cabal file"
    if is_stack; then outdated_nothing "Stack has no outdated report"; fi
    outdated_run cabal outdated
    outdated_finish cabal
}

# JVM: Maven's versions plugin, at a pinned version; for Gradle, the
# Versions plugin where the build applies it (Gradle has no report of its
# own); sbt has none.
_VERSIONS_MAVEN_PLUGIN_VERSION="2.23.0"
outdated_jvm() {
    if [ -f pom.xml ]; then
        jvm_select_jdk
        echo "+ maven versions:display-dependency-updates"
        (maven -q "org.codehaus.mojo:versions-maven-plugin:${_VERSIONS_MAVEN_PLUGIN_VERSION}:display-dependency-updates" \
            -DprocessDependencyManagement=false -Dversions.outputFile="${_outdated_report_file}" \
            -DoutputEncoding=UTF-8 >/dev/null) ||
            { echo "error: ${HECATRON_LANG:-}: the versions plugin failed" >&2; exit 1; }
        outdated_finish maven
    fi
    if is_gradle || nearest_with settings.gradle settings.gradle.kts >/dev/null; then
        local _root _report
        _root="$(nearest_with settings.gradle settings.gradle.kts || pwd)"
        grep -rqs --include='*.gradle' --include='*.gradle.kts' --include='*.toml' 'com.github.ben-manes.versions' "${_root}" ||
            outdated_nothing "Gradle has no outdated report of its own; the Versions plugin (com.github.ben-manes.versions) adds one"
        jvm_select_jdk
        _report="$(mktemp -d)"
        gradle_ dependencyUpdates -DoutputFormatter=json -DoutputDir="${_report}" >/dev/null ||
            { echo "error: ${HECATRON_LANG:-}: gradle dependencyUpdates failed" >&2; exit 1; }
        cp "${_report}/report.json" "${_outdated_report_file}"
        rm -rf "${_report}"
        outdated_finish gradle
    fi
    [ -f build.sbt ] && outdated_nothing "sbt has no outdated report of its own"
    outdated_nothing "no pom.xml or Gradle build"
}

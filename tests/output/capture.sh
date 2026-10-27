#!/usr/bin/env bash
# Recaptures the recorded tool output in tests/output/<tool>/<version>/
# (docs/design.md, "When a tool's output changes"): each fixture under
# fixtures/ fails on purpose, and the log hecatron writes for it is the sample.
#
#   tests/output/capture.sh <image> [case...]
#
# runs every case (or the ones named) in a container of <image>, which needs
# the toolchains installed; Gradle and Maven are downloaded when it lacks them.
# The checkout is mounted read-only and each fixture copied to a writable
# directory. Samples land in tests/output/, or in CAPTURE_OUT when set.
# javac-error and kotlinc-error have no hook that runs the bare compiler, so
# they are the compiler's own output.
set -euo pipefail

_GRADLE_VERSION="9.1.0"
_MAVEN_VERSION="3.9.11"

# Each case: its fixture, language, hecatron action (or "direct" for a bare
# compiler) and the tool whose output it is.
cases() {
    cat <<'EOF'
go-test-fail go test go
go-build-error go build go
rust-test-fail rust test cargo
rust-build-error rust build cargo
python-test-fail python test pytest
python-collect-error python test pytest
tsc-type-error typescript build tsc
vitest-test-fail typescript test vitest
jest-test-fail javascript test jest
node-test-fail javascript test node-test
gradle-junit-fail java test gradle
gradle-javac-error java build gradle
maven-junit-fail java test maven
maven-javac-error java build maven
javac-error java direct javac
kotlin-test-fail kotlin test gradle
kotlin-compile-error kotlin build gradle
kotlinc-error kotlin direct kotlinc
dotnet-test-fail csharp test dotnet
dotnet-build-error csharp build dotnet
rspec-test-fail ruby test rspec
minitest-test-fail ruby test minitest
phpunit-test-fail php test phpunit
exunit-test-fail elixir test mix
mix-compile-error elixir build mix
c-gcc-build-error c build gcc
c-clang-build-error c build clang
cpp-gcc-build-error cpp build gcc
cpp-clang-build-error cpp build clang
EOF
}

# The tool's version as it reports it, run in the case's component directory.
tool_version() {
    case "$1" in
        go) go env GOVERSION | sed 's/^go//' ;;
        cargo) cargo --version | awk '{ print $2 }' ;;
        pytest) uv run --with pytest pytest --version 2>&1 | awk '/^pytest/ { print $2 }' ;;
        tsc) node_modules/.bin/tsc --version | awk '{ print $2 }' ;;
        vitest) node_modules/.bin/vitest --version | sed 's|^vitest/||; s| .*||' ;;
        jest) node_modules/.bin/jest --version ;;
        node-test) node --version | sed 's/^v//' ;;
        gradle) gradle --version | awk '/^Gradle / { print $2 }' ;;
        maven) mvn -v | awk '/^Apache Maven/ { print $3 }' ;;
        javac) javac -version 2>&1 | awk '{ print $2 }' ;;
        kotlinc) kotlinc -version 2>&1 | sed -n 's/.*kotlinc-jvm \([^ ]*\).*/\1/p' ;;
        dotnet) dotnet --version ;;
        rspec) bundle exec rspec --version | sed -n 's/.*rspec-core \([^ ]*\).*/\1/p' ;;
        minitest) ruby -e 'require "minitest"; puts Minitest::VERSION' ;;
        phpunit) vendor/bin/phpunit --version | awk '/^PHPUnit/ { print $2 }' ;;
        mix) elixir -v | awk '/^Elixir/ { print $2 }' ;;
        gcc) gcc -dumpfullversion ;;
        clang) clang -dumpversion ;;
    esac
}

# Gradle or Maven from its release archive, when the image has none.
ensure_build_tools() {
    local _tools="${HOME}/capture-tools"
    if ! command -v gradle >/dev/null 2>&1; then
        mkdir -p "${_tools}"
        curl -fsSL -o "${_tools}/gradle.zip" "https://services.gradle.org/distributions/gradle-${_GRADLE_VERSION}-bin.zip"
        (cd "${_tools}" && python3 -m zipfile -e gradle.zip . && chmod +x "gradle-${_GRADLE_VERSION}/bin/gradle")
        export PATH="${_tools}/gradle-${_GRADLE_VERSION}/bin:${PATH}"
    fi
    if ! command -v mvn >/dev/null 2>&1; then
        mkdir -p "${_tools}"
        curl -fsSL "https://archive.apache.org/dist/maven/maven-3/${_MAVEN_VERSION}/binaries/apache-maven-${_MAVEN_VERSION}-bin.tar.gz" |
            tar -xz -C "${_tools}"
        export PATH="${_tools}/apache-maven-${_MAVEN_VERSION}/bin:${PATH}"
    fi
}

# The bare compiler's combined output, for the cases no hook runs.
run_direct() {
    case "$1" in
        javac-error) javac -d /tmp/javac-out calc/Calc.java ;;
        kotlinc-error) kotlinc Calc.kt -d /tmp/kotlinc-out ;;
    esac
}

# One case: its sample, into <out>/<tool>/<version>/<case>.log.
capture_case() {
    local _case="$1" _lang="$2" _action="$3" _tool="$4" _out="$5"
    local _project="/tmp/capture/${_case}" _log _dir _version
    case "${_tool}" in gradle | maven) ensure_build_tools ;; esac
    mkdir -p /tmp/capture
    cp -R "/hecatron/tests/output/fixtures/${_case}" "${_project}"
    echo "--- ${_case}" >&2
    if [ "${_action}" = direct ]; then
        _log="${_project}/direct.log"
        (cd "${_project}" && run_direct "${_case}" </dev/null >"${_log}" 2>&1) || true
        _dir="${_project}"
    else
        (cd "${_project}" && git init -q && /hecatron/bin/hecatron "${_action}" "${_lang}" </dev/null >/dev/null 2>&1) || true
        _log="${_project}/.logs/${_action}/${_lang}.log"
        _dir="${_project}/${_lang}"
    fi
    _version="$(cd "${_dir}" && tool_version "${_tool}" </dev/null 2>/dev/null | head -1)"
    if [ ! -f "${_log}" ] || [ -z "${_version}" ]; then
        echo "error: ${_case}: no log or no ${_tool} version" >&2
        return 1
    fi
    mkdir -p "${_out}/${_tool}/${_version}"
    cp "${_log}" "${_out}/${_tool}/${_version}/${_case}.log"
    echo "${_tool}/${_version}/${_case}.log" >&2
}

# In the container: every case asked for, then the samples handed back to
# the host user who owns the output directory.
inside() {
    local _out="$1" _case _lang _action _tool _failed=0
    shift
    while read -r _case _lang _action _tool; do
        if [ "$#" -gt 0 ] && ! printf '%s\n' "$@" | grep -qx "${_case}"; then continue; fi
        capture_case "${_case}" "${_lang}" "${_action}" "${_tool}" "${_out}" || _failed=1
    done < <(cases)
    chown -R "$(stat -c %u:%g "${_out}")" "${_out}"
    return "${_failed}"
}

main() {
    if [ "${1:-}" = --inside ]; then
        shift
        inside "$@"
        return
    fi
    [ "$#" -ge 1 ] || {
        echo "usage: $0 <image> [case...]" >&2
        exit 2
    }
    local _image="$1" _here _checkout _out
    shift
    _here="$(cd "$(dirname "$0")" && pwd)"
    _checkout="$(cd "${_here}/../.." && pwd)"
    _out="${CAPTURE_OUT:-${_here}}"
    mkdir -p "${_out}"
    nice -n 10 docker run --rm --cpus 10 --memory 12g \
        -v "${_checkout}:/hecatron:ro" -v "${_out}:/out" \
        "${_image}" bash /hecatron/tests/output/capture.sh --inside /out "$@"
}

main "$@"

#!/usr/bin/env bash
# Recaptures the tools' own test reports hecatron merges into its JUnit
# (docs/guide/runner.md, "Reports") in tests/reports/<tool>/<version>/: from the
# failing fixtures in tests/output/fixtures/, go test -json's events,
# pytest's --junitxml, and the TEST-*.xml Maven's Surefire and Gradle
# write. internal/reports' tests read them.
#
#   tests/reports/capture.sh <image> [case...]
#
# runs every case (or the ones named) in a container of <image>, which needs
# the toolchains installed (tests/output/capture.sh's image). Each case also
# runs hct test with HECATRON_REPORT_DIR, from a binary built here, and
# fails when hecatron's JUnit lacks the tool's test cases. The checkout is
# mounted read-only and each fixture copied to a writable directory.
set -euo pipefail

_GRADLE_VERSION="9.1.0"
_MAVEN_VERSION="3.9.11"

# Each case: its fixture, language, the tool, and a test case's name the
# merged JUnit must contain.
cases() {
    cat <<'EOF'
go-test-fail go go TestAdd
python-test-fail python pytest test_add
maven-junit-fail java maven adds
gradle-junit-fail java gradle adds
kotlin-test-fail kotlin gradle adds
EOF
}

tool_version() {
    case "$1" in
        go) go env GOVERSION | sed 's/^go//' ;;
        pytest) uv run --with pytest pytest --version 2>&1 | awk '/^pytest/ { print $2 }' ;;
        gradle) gradle --version | awk '/^Gradle / { print $2 }' ;;
        maven) mvn -v | awk '/^Apache Maven/ { print $3 }' ;;
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

# One case: hct test with reports, then the tool's report into
# <out>/<tool>/<version>/<case>.(json|xml).
capture_case() {
    local _case="$1" _lang="$2" _tool="$3" _test="$4" _out="$5"
    local _project="/tmp/capture/${_case}" _version _dest _file
    case "${_tool}" in gradle | maven) ensure_build_tools ;; esac
    mkdir -p /tmp/capture
    cp -R "/hecatron/tests/output/fixtures/${_case}" "${_project}"
    echo "--- ${_case}" >&2
    (cd "${_project}" && git init -q &&
        HECATRON_HOME=/hecatron HECATRON_REPORT_DIR="${_project}/reports" GITHUB_STEP_SUMMARY="${_project}/summary.md" \
            /hct test "${_lang}" --output agent </dev/null >"${_project}/agent.txt" 2>&1) || true
    if ! grep -q "name=\"[^\"]*${_test}" "${_project}/reports/test-${_lang}.junit.xml" 2>/dev/null; then
        echo "error: ${_case}: hecatron's JUnit has no ${_test}" >&2
        cat "${_project}/agent.txt" "${_project}/reports/test-${_lang}.junit.xml" >&2 || true
        return 1
    fi
    _version="$(cd "${_project}/${_lang}" && tool_version "${_tool}" </dev/null 2>/dev/null | head -1)"
    [ -n "${_version}" ] || { echo "error: ${_case}: no ${_tool} version" >&2; return 1; }
    _dest="${_out}/${_tool}/${_version}"
    mkdir -p "${_dest}"
    case "${_tool}" in
        go) (cd "${_project}/go" && go test -json ./... >"${_dest}/${_case}.json" 2>/dev/null) || true ;;
        pytest) (cd "${_project}/python" && uv run --with pytest pytest --junitxml="${_dest}/${_case}.xml" >/dev/null 2>&1) || true ;;
        *)
            _file="$(find "${_project}/${_lang}" -name 'TEST-*.xml' \( -path '*surefire-reports*' -o -path '*test-results*' \) | sort | head -1)"
            [ -n "${_file}" ] || { echo "error: ${_case}: no TEST-*.xml" >&2; return 1; }
            cp "${_file}" "${_dest}/${_case}.xml" ;;
    esac
    echo "${_tool}/${_version}/${_case}" >&2
}

inside() {
    local _out="$1" _case _lang _tool _test _failed=0
    shift
    while read -r _case _lang _tool _test; do
        if [ "$#" -gt 0 ] && ! printf '%s\n' "$@" | grep -qx "${_case}"; then continue; fi
        capture_case "${_case}" "${_lang}" "${_tool}" "${_test}" "${_out}" || _failed=1
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
    _bin="$(mktemp -d)" # global: the EXIT trap reads it
    trap 'rm -rf "${_bin}"' EXIT
    (cd "${_checkout}" && CGO_ENABLED=0 GOOS=linux go build -o "${_bin}/hct" ./cmd/hecatron)
    mkdir -p "${_out}"
    nice -n 10 docker run --rm --cpus 10 --memory 12g \
        -v "${_checkout}:/hecatron:ro" -v "${_out}:/out" -v "${_bin}/hct:/hct:ro" \
        "${_image}" bash /hecatron/tests/reports/capture.sh --inside /out "$@"
}

main "$@"

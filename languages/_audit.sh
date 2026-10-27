# Shared helpers for the default audit hooks (languages/<lang>/hooks/audit.sh;
# sourced after _hooks.sh). Each runs its ecosystem's dependency auditor in
# JSON mode and hands the report to libexec/lib/audit.py (or hecatron's
# binary, which reads it alike: audit_reader), which prints the
# findings and a summary, applies the project's accepted risks and fails at
# the threshold (docs/audit.md). Auditors read advisory databases online.
# shellcheck shell=bash
# shellcheck disable=SC2154 # build_dir is set by find_build in _hooks.sh

set -euo pipefail

_audit_home="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
_audit_report_file="$(mktemp)"
_audit_scratch=""
_audit_cleanup() {
    rm -f "${_audit_report_file}"
    if [ -n "${_audit_scratch}" ]; then rm -rf "${_audit_scratch}"; fi
}
trap _audit_cleanup EXIT

# A scratch directory for a lock file resolved from the manifest alone.
audit_scratch() {
    [ -n "${_audit_scratch}" ] || _audit_scratch="$(mktemp -d)"
    echo "${_audit_scratch}"
}

# No lock file: what gets installed changes over time, so this audit is of
# today's resolution, not of what the project last ran.
audit_approximate() {
    echo "warning: ${HECATRON_LANG:-}: no $1, so this audits the versions the manifest resolves to today"
}

# The auditor isn't installed: say how to get it, and fail.
audit_missing() {
    echo "error: ${HECATRON_LANG:-}: $1 isn't installed (hecatron install ${HECATRON_LANG:-<lang>} installs it)" >&2
    exit 1
}

# Run an auditor, its JSON report into the report file; its messages pass
# through. Exits non-zero for findings as well as errors, so the status
# isn't read: a report audit.py can't parse is the error.
audit_run() {
    echo "+ $*"
    "$@" >"${_audit_report_file}" || true
}

# libexec/lib/audit.py's commands, run by hecatron's binary when the run
# commands give it (HECATRON_BINARY: the same reader, without Python).
audit_reader() {
    if [ -n "${HECATRON_BINARY:-}" ]; then
        "${HECATRON_BINARY}" _audit "$@"
    else
        python3 "${_audit_home}/libexec/lib/audit.py" "$@"
    fi
}

# The findings in the report file, in a format audit.py reads; then the
# toolchain's support status. Exits with the audit's status.
audit_finish() {
    local _status=0
    if [ -n "${HECATRON_AUDIT_REPORT_DIR:-}" ]; then
        mkdir -p "${HECATRON_AUDIT_REPORT_DIR}"
        cp "${_audit_report_file}" "${HECATRON_AUDIT_REPORT_DIR}/${HECATRON_LANG:-}-${HECATRON_COMPONENT:-}-$1.json"
    fi
    audit_reader report "$1" "${_audit_report_file}" "${HECATRON_LANG:-}" "${HECATRON_COMPONENT:-.}" || _status=$?
    audit_toolchain
    exit "${_status}"
}

# A warning when the component's toolchain is past its end of life, from
# the definition's releases.sh (which asks upstream).
audit_toolchain() {
    local _releases _version
    _releases="$(python3 "${_audit_home}/libexec/lib/project.py" resolve "${HECATRON_LANG:-}" releases.sh 2>/dev/null)" ||
        return 0
    _version="$(python3 "${_audit_home}/libexec/lib/project.py" version "${HECATRON_LANG:-}" "${HECATRON_COMPONENT:-}" \
        2>/dev/null | cut -f3)"
    [ -n "${_version}" ] && [ "${_version}" != - ] || return 0
    bash "${_releases}" 2>/dev/null | audit_reader eol "${HECATRON_LANG:-}" "${_version}" || true
}

# Nothing this auditor can read here: say so, and succeed.
audit_nothing() {
    echo "${HECATRON_LANG:-}: audit: nothing to audit: $*"
    exit 0
}

# Node.js: the package manager's own audit (npm, pnpm, Yarn or Bun). Without
# a lock file, npm resolves one from package.json in a scratch directory.
audit_node() {
    find_build --above-first package.json || audit_nothing "no package.json"
    note_build_dir
    cd "${build_dir}"
    # shellcheck source=languages/typescript/package-manager.sh
    source "${_audit_home}/languages/typescript/package-manager.sh"
    node_pm_command
    local _lock=""
    case "${node_pm_name}" in
        pnpm) [ -f pnpm-lock.yaml ] && _lock=pnpm-lock.yaml ;;
        yarn) [ -f yarn.lock ] && _lock=yarn.lock ;;
        bun) { [ -f bun.lock ] || [ -f bun.lockb ]; } && _lock=bun.lock ;;
        *) { [ -f package-lock.json ] || [ -f npm-shrinkwrap.json ]; } && _lock=package-lock.json ;;
    esac
    if [ -z "${_lock}" ]; then
        audit_approximate "lock file"
        local _scratch
        _scratch="$(audit_scratch)"
        cp package.json "${_scratch}/"
        (cd "${_scratch}" && run npm install --package-lock-only --ignore-scripts --no-audit --no-fund >/dev/null) ||
            { echo "error: ${HECATRON_LANG:-}: npm couldn't resolve package.json's dependencies" >&2; exit 1; }
        cd "${_scratch}"
        audit_run npm audit --json
        audit_finish npm
    fi
    case "${node_pm_name}" in
        pnpm)
            audit_run "${node_pm[@]}" audit --json
            audit_finish pnpm ;;
        yarn)
            if [ "$("${node_pm[@]}" --version 2>/dev/null | cut -d. -f1)" = 1 ]; then
                audit_run "${node_pm[@]}" audit --json
                audit_finish yarn-classic
            fi
            audit_run "${node_pm[@]}" npm audit --all --recursive --json
            audit_finish yarn-berry ;;
        bun)
            audit_run bun audit --json
            audit_finish bun ;;
        *)
            audit_run npm audit --json
            audit_finish npm ;;
    esac
}

# Python: pip-audit (PyPA's), run through uvx so nothing is installed. A uv
# lock is exported to pinned requirements; pinned requirements files are
# read as they are; otherwise pip-audit resolves the project's metadata.
_PIP_AUDIT_VERSION="2.10.1"
audit_python() {
    has uvx || audit_missing uv
    local _pip_audit=(uvx "pip-audit@${_PIP_AUDIT_VERSION}") _requirements
    if find_build --above-first uv.lock; then
        note_build_dir
        cd "${build_dir}"
        _requirements="$(audit_scratch)/requirements.txt"
        run uv export --frozen --format requirements-txt --no-hashes --no-emit-project --quiet -o "${_requirements}" ||
            { echo "error: python: uv couldn't export uv.lock" >&2; exit 1; }
        audit_run "${_pip_audit[@]}" --progress-spinner off -f json --no-deps --disable-pip -r "${_requirements}"
        audit_finish pip-audit
    fi
    if exists 'requirements*.txt'; then
        local _args=() _file
        for _file in requirements*.txt; do _args+=(-r "${_file}"); done
        grep -qE '==' requirements*.txt || audit_approximate "pinned requirements"
        audit_run "${_pip_audit[@]}" --progress-spinner off -f json "${_args[@]}"
        audit_finish pip-audit
    fi
    if has_any pyproject.toml setup.py; then
        audit_approximate "lock file (uv.lock) or pinned requirements"
        audit_run "${_pip_audit[@]}" --progress-spinner off -f json .
        audit_finish pip-audit
    fi
    audit_nothing "no uv.lock, requirements file, pyproject.toml or setup.py"
}

# Go: govulncheck (the Go team's), which reports only the vulnerabilities
# the code calls. Run at a pinned version through go run, so nothing is
# installed. It analyses the standard library too, so it needs a release
# that knows the Go it runs on.
_GOVULNCHECK_VERSION="v1.9.0"
audit_go() {
    find_build --above-first go.mod || audit_nothing "no go.mod"
    note_build_dir
    cd "${build_dir}"
    audit_run go run "golang.org/x/vuln/cmd/govulncheck@${_GOVULNCHECK_VERSION}" -json ./...
    audit_finish govulncheck
}

# Ruby: bundler-audit against Gemfile.lock, its advisory database updated
# first. Without a lock, Bundler resolves one in a scratch directory.
audit_ruby() {
    has bundle-audit || audit_missing bundle-audit
    find_build --above-first Gemfile || audit_nothing "no Gemfile"
    note_build_dir
    cd "${build_dir}"
    local _gemfile_lock=Gemfile.lock
    if [ ! -f Gemfile.lock ]; then
        audit_approximate Gemfile.lock
        local _scratch
        _scratch="$(audit_scratch)"
        cp Gemfile "${_scratch}/"
        (cd "${_scratch}" && BUNDLE_GEMFILE="${_scratch}/Gemfile" run bundle lock >/dev/null) ||
            { echo "error: ruby: Bundler couldn't resolve the Gemfile" >&2; exit 1; }
        _gemfile_lock="${_scratch}/Gemfile.lock"
    fi
    run bundle-audit update --quiet || true
    audit_run bundle-audit check --gemfile-lock "${_gemfile_lock}"
    cat "${_audit_report_file}"
    audit_finish bundle-audit
}

# PHP: Composer's own audit of composer.lock. Without a lock, Composer
# resolves one in a scratch directory, installing nothing.
audit_php() {
    find_build --above-first composer.json || audit_nothing "no composer.json"
    note_build_dir
    cd "${build_dir}"
    if [ ! -f composer.lock ]; then
        audit_approximate composer.lock
        local _scratch
        _scratch="$(audit_scratch)"
        cp composer.json "${_scratch}/"
        cd "${_scratch}"
        # Composer 2.9 refuses to resolve to versions with advisories; the
        # audit is what should report them.
        composer config audit.block-insecure false 2>/dev/null || true
        run composer update --no-install --no-scripts --no-plugins --no-interaction --ignore-platform-reqs ||
            { echo "error: php: Composer couldn't resolve composer.json" >&2; exit 1; }
    fi
    audit_run composer audit --locked --format=json --no-interaction
    audit_finish composer
}

# .NET: the SDK's own audit of every package, direct and transitive, after
# a restore.
audit_dotnet() {
    find_build '*.sln' '*.slnx' '*.csproj' '*.fsproj' '*.vbproj' ||
        audit_nothing "no .sln, .slnx or project file"
    note_build_dir
    cd "${build_dir}"
    local _target
    _target="$(dotnet_target)"
    run dotnet restore ${_target:+"${_target}"} >/dev/null ||
        { echo "error: ${HECATRON_LANG:-}: dotnet restore failed" >&2; exit 1; }
    audit_run dotnet list ${_target:+"${_target}"} package --vulnerable --include-transitive --format json
    audit_finish dotnet
}

# Everything else: OSV-Scanner (Google's) on the lock files and manifests it
# reads in this directory and below (Cargo.lock, pom.xml, gradle.lockfile,
# pubspec.lock, mix.lock, Package.resolved, renv.lock, cabal.project.freeze,
# stack.yaml.lock, conan.lock...).
audit_osv() {
    has osv-scanner || audit_missing osv-scanner
    local _status=0
    echo "+ osv-scanner scan source --recursive --format json ."
    osv-scanner scan source --recursive --format json . >"${_audit_report_file}" || _status=$?
    # 1 is findings; 128, no package sources found.
    [ "${_status}" -ne 128 ] || audit_nothing "no lock file or manifest OSV-Scanner reads"
    audit_finish osv
}

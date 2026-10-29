#!/usr/bin/env bash
# Which of CI's jobs (.github/workflows/ci.yml) a pull request can affect,
# from the paths it changes, one per line on stdin:
#
#   git diff --name-only --no-renames BASE HEAD | libexec/maintenance/ci-jobs.sh
#   libexec/maintenance/ci-jobs.sh --all     # every job (pushes to main)
#
# Prints a line per job, "<job>=true" or "<job>=false", then "test_os=<JSON
# list>" for the test job's matrix, ready to append to $GITHUB_OUTPUT.
#
# Conservative: a path this doesn't know runs every job, and so does any
# change to the workflows. Documentation alone (docs/, the top-level .md
# files, changelog.d/, assets/, llms.txt) runs only the docs job. ci-ok
# always runs and passes when every selected job did.
set -euo pipefail

_jobs="
docs
test_ubuntu
test_macos
self_hosted
go
bash_3_2
installer_linux
installer_macos
install_parity
"

# The languages install-parity installs (keep in step with ci.yml).
_parity_languages="
bash
raku
racket
zig
"

for _job in ${_jobs}; do
    eval "_run_${_job}=false"
done

_select() {
    local _job
    for _job in "$@"; do
        eval "_run_${_job}=true"
    done
}

_all() {
    local _job
    for _job in ${_jobs}; do
        _select "${_job}"
    done
}

# Everything that runs the Python suite (and shellcheck) on Linux and macOS
# and under bash 3.2, and the Go tests, which read bin/, libexec/,
# languages/ and the test fixtures.
_scripts() {
    _select test_ubuntu test_macos bash_3_2 go
}

_parity_language() {
    local _lang
    for _lang in ${_parity_languages}; do
        [ "$1" = "${_lang}" ] && return 0
    done
    return 1
}

_classify() {
    local _path="$1" _lang
    case "${_path}" in
        "") ;;
        .github/*)
            _all ;;

        # The Go core. internal/ also holds golden files the Python suite
        # compares with the scripts' output (test_reports, test_generated_ci).
        # self-hosted is the only job that runs golangci-lint.
        go.mod | go.sum)
            _select go self_hosted install_parity ;;
        cmd/hecatron/install* | cmd/hecatron/doctor*)
            _select go self_hosted install_parity ;;
        cmd/*)
            _select go self_hosted ;;
        internal/install/*)
            _select go self_hosted install_parity test_ubuntu bash_3_2 ;;
        internal/*.go)
            _select go self_hosted ;;
        internal/*)
            _select go self_hosted test_ubuntu bash_3_2 ;;

        # The launcher, which the installers end by running.
        bin/*)
            _scripts
            _select self_hosted installer_linux installer_macos ;;
        install.sh | install.ps1)
            _select test_ubuntu self_hosted installer_linux installer_macos ;;
        VERSION | .hecatron/* | .gitignore)
            _all ;;

        # libexec/lib/ is sourced by the installers parity runs.
        libexec/lib/*)
            _scripts
            _select self_hosted install_parity ;;
        libexec/commands/version.sh)
            _scripts
            _select self_hosted installer_linux installer_macos ;;
        libexec/*)
            _scripts
            _select self_hosted ;;

        # Language definitions. self-hosted installs Go with them; the shared
        # helpers (languages/_*) are in every installer.
        languages/_*)
            _scripts
            _select self_hosted install_parity ;;
        languages/go/*)
            _scripts
            _select self_hosted ;;
        languages/*/install.json | languages/*/install.sh)
            _scripts
            _select install_parity ;;
        languages/*/*)
            _scripts
            _lang="${_path#languages/}"
            _lang="${_lang%%/*}"
            if _parity_language "${_lang}"; then
                _select install_parity
            fi ;;
        languages/*)
            _scripts ;;

        # The tests. tests/*.py drive the shell scripts, so they run where the
        # scripts do (macOS and bash 3.2 included); the subdirectories hold
        # the Go tests' fixtures and the Docker tests.
        tests/docker/run.sh | tests/docker/Dockerfile)
            _scripts
            _select install_parity installer_linux ;;
        tests/docker/* | tests/install/*)
            _scripts
            _select install_parity ;;
        tests/installer/*)
            _scripts
            _select installer_linux ;;
        tests/*/*)
            _scripts ;;
        tests/*)
            _select test_ubuntu test_macos bash_3_2 ;;

        # The Agent Skill: the Go binary serves it, the Python suite checks
        # its generated parts.
        plugins/* | .claude-plugin/*)
            _select test_ubuntu go ;;
        # Reference material some Python tests read.
        examples/*)
            _select test_ubuntu test_macos bash_3_2 ;;

        # Documentation. The Go advisor's tests read docs/guide/advisor.md.
        docs/guide/advisor.md)
            _select docs go ;;
        docs/* | changelog.d/* | assets/* | llms.txt)
            _select docs ;;
        */*)
            _all ;;
        *.md)
            _select docs ;;

        *)
            _all ;;
    esac
}

if [ "${1:-}" = "--all" ]; then
    _all
elif [ "$#" -gt 0 ]; then
    echo "usage: $0 [--all] < changed-paths" >&2
    exit 2
else
    while IFS= read -r _path || [ -n "${_path}" ]; do
        _classify "${_path}"
    done
fi

_selected() {
    eval "[ \"\${_run_$1}\" = true ]"
}

for _job in ${_jobs}; do
    if _selected "${_job}"; then echo "${_job}=true"; else echo "${_job}=false"; fi
done

_os=""
if _selected test_ubuntu; then
    _os='"ubuntu-latest"'
fi
if _selected test_macos; then
    _os="${_os:+${_os},}\"macos-latest\""
fi
echo "test_os=[${_os}]"

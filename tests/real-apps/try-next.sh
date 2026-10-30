#!/usr/bin/env bash
# The real-apps workflow (.github/workflows/real-apps.yml): build the next
# open-source Xcode app on apps.txt with hct as a newcomer would, and file
# an issue when something goes wrong. No judgement: report.py reports what
# the run recorded.
#
#   try-next.sh build WORK    clone, hct init, hct deps, hct build; records into WORK
#   try-next.sh report WORK   files or updates the app's issue, writes the job summary
#
# build runs the app's own build scripts, so the workflow runs it with no
# token that can write; report runs none of the app's code. Environment:
# APP (owner/repo; default: whose turn it is), DRY_RUN=1 (report prints the
# issue instead of filing it), GH_TOKEN and GITHUB_* (Actions).

set -uo pipefail

_here="$(cd "$(dirname "$0")" && pwd)"
_home="$(cd "${_here}/../.." && pwd)"
_report=(python3 "${_here}/report.py")
_label="real-app"

# A fact about the run, for report.py.
meta() { printf '%s\t%s\n' "$1" "$2" >>"${_work}/meta.tsv"; }

# Run a step: its output in logs/<name>.log, its status and time in
# steps.tsv. The second argument is the command as the report shows it.
step() {
    local _name="$1" _shown="$2" _start _status=0
    shift 2
    echo "==> ${_name}: ${_shown}"
    _start="$(date +%s)"
    "$@" >"${_work}/logs/${_name}.log" 2>&1 || _status=$?
    printf '%s\t%s\t%s\t%s\n' "${_name}" "${_status}" "$(($(date +%s) - _start))" "${_shown}" >>"${_work}/steps.tsv"
    echo "    exit ${_status}"
    return "${_status}"
}

# hct build's JSON report into build.json; its log goes to the step's.
hct_build() { "${_hct}" build "$@" --output json --timeout 45m >"${_work}/build.json"; }

build() {
    mkdir -p "${_work}/logs"
    : >"${_work}/meta.tsv"
    : >"${_work}/steps.tsv"
    local _app _langs=() _lang
    _app="${APP:-$("${_report[@]}" pick "${_here}/apps.txt")}"
    meta app "${_app}"
    meta hecatron_sha "$(git -C "${_home}" rev-parse HEAD)"
    meta macos "$(sw_vers -productVersion 2>/dev/null || uname -sr)"
    meta xcode "$(xcodebuild -version 2>/dev/null | tr '\n' ' ' | sed 's/ *$//')"
    if [ -n "${GITHUB_RUN_ID:-}" ]; then
        meta run_url "${GITHUB_SERVER_URL:-https://github.com}/${GITHUB_REPOSITORY:-}/actions/runs/${GITHUB_RUN_ID}"
    fi

    # The binary, at hecatron's own checkout (HECATRON_HOME).
    _hct="${_work}/hct"
    if ! (cd "${_home}" && go build -o "${_hct}" ./cmd/hecatron); then
        meta harness_error "hecatron's binary didn't build"
        return 1
    fi
    export HECATRON_HOME="${_home}"

    step clone "git clone --depth 1 --recurse-submodules --shallow-submodules https://github.com/${_app}" \
        git clone --depth 1 --recurse-submodules --shallow-submodules "https://github.com/${_app}.git" "${_work}/app" ||
        return 0
    meta app_sha "$(git -C "${_work}/app" rev-parse HEAD)"
    cd "${_work}/app" || return 1
    if step init "hct init --no-agents" "${_hct}" init --no-agents; then
        while read -r _lang _; do
            case "${_lang}" in
                swift | objectivec) _langs+=("${_lang}") ;;
            esac
        done < <("${_hct}" list 2>/dev/null)
        meta languages "${_langs[*]+"${_langs[*]}"}"
        if [ "${#_langs[@]}" -gt 0 ]; then
            "${_hct}" _xcode plan --json . >"${_work}/plan.json" 2>/dev/null || rm -f "${_work}/plan.json"
            step deps "hct deps ${_langs[*]}" "${_hct}" deps "${_langs[@]}" --timeout 30m || true
            step build "hct build ${_langs[*]} --output json" hct_build "${_langs[@]}" || true
        fi
    fi
    if [ -d .logs ]; then
        cp -R .logs "${_work}/logs/hct"
    fi
    return 0
}

# The open issue for the app (its number), if there is one.
open_issue() {
    gh issue list --repo "${GITHUB_REPOSITORY}" --label "${_label}" --state open --limit 200 \
        --json number,title --jq ".[] | select(.title == \"$1\") | .number" | head -n 1
}

report() {
    local _kind _file _line _title _issue _text
    IFS=$'\t' read -r _kind _file _line < <("${_report[@]}" outcome "${_work}")
    echo "${_line}"
    if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
        "${_report[@]}" summary "${_work}" >>"${GITHUB_STEP_SUMMARY}"
    fi
    _text="$(mktemp)"
    "${_report[@]}" issue "${_work}" >"${_text}"
    _title="$(head -n 1 "${_text}")"
    if [ "${DRY_RUN:-}" = 1 ]; then
        echo "--- ${_kind} (${_file}): would file or comment on \"${_title}\":"
        tail -n +2 "${_text}"
        return 0
    fi
    gh label create "${_label}" --repo "${GITHUB_REPOSITORY}" --color 5319e7 \
        --description "Found by the real-apps workflow (tests/real-apps/)" 2>/dev/null || true
    _issue="$(open_issue "${_title}")" || return 1
    if [ -n "${_issue}" ]; then
        "${_report[@]}" comment "${_work}" >"${_text}"
        gh issue comment "${_issue}" --repo "${GITHUB_REPOSITORY}" --body-file "${_text}"
    elif [ "${_file}" = file ]; then
        tail -n +2 "${_text}" >"${_text}.body"
        gh issue create --repo "${GITHUB_REPOSITORY}" --title "${_title}" --label "${_label}" --body-file "${_text}.body"
    fi
}

case "${1:-}" in
    build | report)
        [ -n "${2:-}" ] || { echo "usage: $0 build|report WORK" >&2; exit 2; }
        mkdir -p "$2"
        _work="$(cd "$2" && pwd)"
        "$1" ;;
    *)
        echo "usage: $0 build|report WORK" >&2
        exit 2 ;;
esac

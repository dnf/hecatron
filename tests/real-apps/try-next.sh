#!/usr/bin/env bash
# The real-apps workflow (.github/workflows/real-apps.yml): build the next
# open-source Xcode app on apps.txt with hct as a newcomer would, and file
# an issue when something goes wrong. No judgement: report.py reports what
# the run recorded.
#
#   try-next.sh pick          the app to build: APP, else the oldest whose issue is
#                             labelled real-app-retry, else whose turn it is
#   try-next.sh build WORK    clone, the app's fixups.txt steps, hct init, deps and
#                             build; records into WORK
#   try-next.sh report WORK   files or updates the app's issue, writes the job summary
#
# build runs the app's own build scripts, so the workflow runs it with no
# token at all; pick and report run none of the app's code. Environment:
# APP (owner/repo), RETRY_ISSUE (the issue pick took it from), DRY_RUN=1
# (report prints the issue instead of filing it), GH_TOKEN and GITHUB_*
# (Actions).

set -uo pipefail

_here="$(cd "$(dirname "$0")" && pwd)"
_home="$(cd "${_here}/../.." && pwd)"
_report=(python3 "${_here}/report.py")
_label="real-app"
_retry_label="real-app-retry"

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

# The app and, when it's a retry, its issue: "app<TAB>issue", and the
# step's outputs for the workflow.
pick() {
    local _app="${APP:-}" _issue="" _found
    if [ -z "${_app}" ] && [ -n "${GH_TOKEN:-}" ]; then
        _found="$(gh issue list --repo "${GITHUB_REPOSITORY}" --label "${_label}" --label "${_retry_label}" \
            --state open --limit 100 --json number,title,createdAt \
            --jq 'sort_by(.createdAt) | map(select(.title | startswith("Real app: "))) | first // empty |
                  "\(.title | ltrimstr("Real app: "))\t\(.number)"')" || _found=""
        if [ -n "${_found}" ]; then
            _app="${_found%%$'\t'*}"
            _issue="${_found#*$'\t'}"
        fi
    fi
    [ -n "${_app}" ] || _app="$("${_report[@]}" pick "${_here}/apps.txt")"
    printf '%s\t%s\n' "${_app}" "${_issue}"
    if [ -n "${GITHUB_OUTPUT:-}" ]; then
        printf 'app=%s\nretry_issue=%s\n' "${_app}" "${_issue}" >>"${GITHUB_OUTPUT}"
    fi
}

# The app's setup steps from fixups.txt, as one command for bash -c.
setup_command() {
    local _step _command=""
    while IFS= read -r _step; do
        _command="${_command:+${_command} && }${_step}"
    done < <("${_report[@]}" fixups "${_here}/fixups.txt" "$1")
    printf '%s' "${_command}"
}

# hct build's JSON report into build.json; its log goes to the step's.
hct_build() { "${_hct}" build "$@" --output json --timeout 45m >"${_work}/build.json"; }

build() {
    mkdir -p "${_work}/logs"
    : >"${_work}/meta.tsv"
    : >"${_work}/steps.tsv"
    local _app _langs=() _lang _setup
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
    _setup="$(setup_command "${_app}")"
    if [ -n "${_setup}" ]; then
        step setup "${_setup}" bash -c "${_setup}" || return 0
    fi
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
    gh label create "${_retry_label}" --repo "${GITHUB_REPOSITORY}" --color fbca04 \
        --description "The real-apps workflow tries this app next" 2>/dev/null || true
    _issue="$(open_issue "${_title}")" || return 1
    if [ -n "${_issue}" ]; then
        "${_report[@]}" comment "${_work}" >"${_text}"
        gh issue comment "${_issue}" --repo "${GITHUB_REPOSITORY}" --body-file "${_text}"
        if [ "${_kind}" = passed ]; then
            gh issue close "${_issue}" --repo "${GITHUB_REPOSITORY}"
        fi
    elif [ "${_file}" = file ]; then
        tail -n +2 "${_text}" >"${_text}.body"
        gh issue create --repo "${GITHUB_REPOSITORY}" --title "${_title}" --label "${_label}" --body-file "${_text}.body"
    fi
    # The retry is done, whichever way it went; labelling it again retries
    # it again.
    if [ -n "${RETRY_ISSUE:-}" ]; then
        gh issue edit "${RETRY_ISSUE}" --repo "${GITHUB_REPOSITORY}" --remove-label "${_retry_label}"
    fi
}

case "${1:-}" in
    pick)
        pick ;;
    build | report)
        [ -n "${2:-}" ] || { echo "usage: $0 build|report WORK" >&2; exit 2; }
        mkdir -p "$2"
        _work="$(cd "$2" && pwd)"
        "$1" ;;
    *)
        echo "usage: $0 pick | build WORK | report WORK" >&2
        exit 2 ;;
esac

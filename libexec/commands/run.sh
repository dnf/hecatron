#!/usr/bin/env bash
# hecatron test|lint|build|clean|rebuild|retest|audit|format|deps|outdated|docs|bench [-j N] [-s MODE] [-c COMPONENT]... [target ...]
#
# With targets, runs each one in turn, streaming its output and saving it to
# <logs>/<action>/<target>.log. A target is a language (every component), a
# project task (config.json "tasks"), or for lint the `shell` tool. Without
# targets, runs every applicable language and task in parallel with the live
# progress display.
set -euo pipefail

# shellcheck source=../lib/helpers.sh
source "${HECATRON_HOME}/libexec/lib/helpers.sh"

_action="${1:?usage: run.sh <action> [flags] [target ...]}"
shift

_jflag="" _sflag="" _prev="" _json=0
# --fail-on-skip, else HECATRON_FAIL_ON_SKIP=1: fail when nothing ran. The
# jobs this run starts mustn't see it: each would fail on its own skip.
_fail_on_skip=0
[ "${HECATRON_FAIL_ON_SKIP:-}" != 1 ] || _fail_on_skip=1
unset HECATRON_FAIL_ON_SKIP
_targets=()
_components=()
for _a in "$@"; do
    if [ "${_prev}" = "-j" ]; then _jflag="${_a}"; _prev=""; continue; fi
    if [ "${_prev}" = "-s" ]; then _sflag="${_a}"; _prev=""; continue; fi
    if [ "${_prev}" = "-c" ]; then _components+=("${_a}"); _prev=""; continue; fi
    if [ "${_prev}" = "--fail-on" ]; then export HECATRON_AUDIT_FAIL_ON="${_a}"; _prev=""; continue; fi
    case "${_a}" in
        -j|-s|-c) _prev="${_a}" ;;
        --fail-on) [ "${_action}" = audit ] && _prev="${_a}" || _targets+=("${_a}") ;;
        --fail-on=*) export HECATRON_AUDIT_FAIL_ON="${_a#--fail-on=}" ;;
        --check) [ "${_action}" = format ] && export HECATRON_FORMAT_CHECK=1 || _targets+=("${_a}") ;;
        --json) [ "${_action}" = outdated ] && _json=1 || _targets+=("${_a}") ;;
        --fail) [ "${_action}" = outdated ] && export HECATRON_OUTDATED_FAIL=1 || _targets+=("${_a}") ;;
        --component=*) _components+=("${_a#--component=}") ;;
        --fail-on-skip) _fail_on_skip=1 ;;
        *) _targets+=("${_a}") ;;
    esac
done
for _component in ${_components[@]+"${_components[@]}"}; do
    if ! _in_list "${_component}" ${HECATRON_COMPONENTS}; then
        echo "hecatron: unknown component '${_component}' (components: ${HECATRON_COMPONENTS})" >&2
        exit 2
    fi
done

_tasks() { python3 "${HECATRON_HOME}/libexec/lib/project.py" tasks "$1"; }
_task_command() { python3 "${HECATRON_HOME}/libexec/lib/project.py" task "$1" "$2"; }

_describe() {
    case "$1" in
        test) echo "Run each language's tests for every component" ;;
        lint) echo "Run each language's linter for every component" ;;
        build) echo "Build each language for every component" ;;
        clean) echo "Remove each language's build artifacts for every component (keeps installed dependencies)" ;;
        rebuild) echo "Clean, then build" ;;
        retest) echo "Clean, then test" ;;
        audit) echo "Check each language's dependencies for known vulnerabilities in every component" ;;
        format) echo "Format each language's code with its formatter in every component" ;;
        deps) echo "Fetch each language's dependencies for every component, held to its lock file" ;;
        outdated) echo "Report newer versions of each language's dependencies in every component" ;;
        docs) echo "Build each language's API documentation for every component" ;;
        bench) echo "Run each language's benchmarks for every component" ;;
    esac
}

# The languages an action applies to.
_action_langs() {
    case "$1" in
        test|retest) echo "${_TEST_LANGS}" ;;
        lint) echo "${_LINT_LANGS}" ;;
        build|rebuild) echo "${_COMPILED_LANGS}" ;;
        clean) echo "${_CLEAN_LANGS}" ;;
        audit) echo "${_AUDIT_LANGS}" ;;
        format) echo "${_FORMAT_LANGS}" ;;
        deps) echo "${_DEPS_LANGS}" ;;
        outdated) echo "${_OUTDATED_LANGS}" ;;
        docs) echo "${_DOCS_LANGS}" ;;
        bench) echo "${_BENCH_LANGS}" ;;
    esac
}

_usage() {
    local tasks
    tasks="$(_tasks "${_action}" | tr '\n' ' ')"
    printf '%s\n' \
        "Usage: hecatron ${_action} [flags] [target ...]" \
        "" \
        "$(_describe "${_action}"). With no targets, runs them all in parallel." \
        ""
    _lang_grid "Languages" "$(_action_langs "${_action}")"
    if [ -n "${tasks}" ]; then
        printf '\nProject tasks: %s\n' "${tasks}"
    fi
    if [ "${_action}" = "lint" ]; then
        printf '\nTools:\n  shell      shellcheck on every tracked shell script\n'
    fi
    if [ "${_action}" = "audit" ]; then
        printf '\nTools:\n  secrets    gitleaks: committed credentials in the working tree (with hecatron'"'"'s binary)\n'
    fi
    printf '%s\n' \
        "" \
        "Flags:" \
        "  -j VALUE   Concurrency: integer, percentage (default 50%), or 'unlimited'" \
        "  -s MODE    Display: scroll (default), full, or zoom" \
        "  -c NAME    Only this component (repeatable); components: ${HECATRON_COMPONENTS}" \
        "  --fail-on-skip" \
        "             Fail when nothing ran: every job skipped (no tests, no tool, not installed);" \
        "             else HECATRON_FAIL_ON_SKIP=1"
    if [ "${_action}" = "audit" ]; then
        printf '%s\n' \
            "  --fail-on SEVERITY" \
            "             Fail on a finding at or above critical, high (default), moderate or low;" \
            "             a project sets its own with \"audit\": {\"failOn\": ...} (docs/guide/audit.md)"
    fi
    if [ "${_action}" = "outdated" ]; then
        printf '%s\n' \
            "  --json     Print one JSON array of every outdated dependency (component, language," \
            "             package, current, wanted, latest); the languages run one at a time" \
            "  --fail     Fail when anything is outdated (by default this only reports)"
    fi
    if [ "${_action}" = "format" ]; then
        printf '%s\n' \
            "  --check    Change nothing; fail, listing the files the formatter would change" \
            "             (hecatron lint runs this check too; docs/guide/format.md)"
    fi
    printf '%s\n' \
        "" \
        "With hecatron's binary (install.sh downloads it with a release; docs/guide/runner.md):" \
        "  --output FORMAT" \
        "             human, agent, json, ci, junit or sarif (else HECATRON_OUTPUT;" \
        "             agent output for AI agents)" \
        "  --timeout DURATION" \
        "             Stop a job after DURATION (90s, 30m, 1h30m; 0: no limit; else HECATRON_TIMEOUT);" \
        "             by default 5x its usual time from history.log, at least 10 minutes"
    if [ "${_action}" = "audit" ]; then
        printf '%s\n' "  --history  The secrets scan reads every commit instead of the working tree"
    fi
    case "${_action}" in
        test|lint|build|format|deps|rebuild|retest|docs|bench)
            printf '%s\n' \
                "  --changed [REF]" \
                "             Only the languages, components and project tasks the files changed" \
                "             since REF touch (default: the pull request's base, else the default branch)"
            ;;
    esac
    printf '%s\n' \
        "" \
        "Examples:" \
        "  hecatron ${_action}               Everything, in parallel" \
        "  hecatron ${_action} go rs         Go and Rust (abbreviations work), every component" \
        "  hecatron ${_action} go -c age     Go, the age component only"
}

if [ "${#_targets[@]}" -gt 0 ] && { [ "${_targets[0]}" = "help" ] || [ "${_targets[0]}" = "--help" ]; }; then
    _usage
    exit 0
fi

cd "${HECATRON_ROOT}"
export HECATRON_JOB_COMMAND="${HECATRON_HOME}/bin/hecatron"

# rebuild/retest: clean, then build/test, for the same targets.
case "${_action}" in
    rebuild|retest)
        _then="build"; [ "${_action}" = "retest" ] && _then="test"
        "${HECATRON_JOB_COMMAND}" clean ${_targets[@]+"${_targets[@]}"}
        _then_flags=()
        [ "${_fail_on_skip}" -eq 0 ] || _then_flags+=(--fail-on-skip)
        exec "${HECATRON_JOB_COMMAND}" "${_then}" ${_jflag:+-j "${_jflag}"} ${_sflag:+-s "${_sflag}"} \
            ${_then_flags[@]+"${_then_flags[@]}"} ${_targets[@]+"${_targets[@]}"}
        ;;
esac

# --- outdated --json: one JSON array on stdout ------------------------------
# Every language, or the ones named, one at a time with their output on
# stderr; each component saves its rows (libexec/lib/outdated.py).
if [ "${_json}" -eq 1 ]; then
    if [ "${#_targets[@]}" -eq 0 ]; then
        for _lang in ${_OUTDATED_LANGS}; do
            if ! _is_disabled "${_lang}" && _dep_available "${_lang}"; then _targets+=("${_lang}"); fi
        done
    fi
    HECATRON_OUTDATED_JSON_DIR="$(mktemp -d)"
    export HECATRON_OUTDATED_JSON_DIR
    trap 'rm -rf "${HECATRON_OUTDATED_JSON_DIR}"' EXIT
    _component_flags=()
    for _component in ${_components[@]+"${_components[@]}"}; do _component_flags+=(-c "${_component}"); done
    _status=0
    for _target in ${_targets[@]+"${_targets[@]}"}; do
        "${HECATRON_JOB_COMMAND}" outdated ${_component_flags[@]+"${_component_flags[@]}"} "${_target}" >&2 || _status=1
    done
    python3 "${HECATRON_HOME}/libexec/lib/outdated.py" collect "${HECATRON_OUTDATED_JSON_DIR}"
    exit "${_status}"
fi

# --- Everything, in parallel ------------------------------------------------
if [ "${#_targets[@]}" -eq 0 ]; then
    _cflags=""
    for _component in ${_components[@]+"${_components[@]}"}; do
        _cflags="${_cflags} -c ${_component}"
    done
    _jobs=()
    for _lang in $(_action_langs "${_action}"); do
        if _is_disabled "${_lang}"; then
            _jobs+=("disable:${_lang}")
        elif _dep_available "${_lang}"; then
            _jobs+=("${_lang}::${_action} ${_lang}${_cflags}")
        else
            _jobs+=("skip:${_lang}")
        fi
    done
    _extra=()
    if [ -z "${_cflags}" ]; then
        while IFS= read -r _task; do
            [ -n "${_task}" ] && _extra+=("${_action} ${_task}")
        done < <(_tasks "${_action}")
    fi
    if [ "${_action}" = "lint" ] && _is_tool shell; then
        if command -v shellcheck >/dev/null 2>&1; then _extra+=("lint shell"); else _extra+=("skip:shell"); fi
    fi
    if [ "${#_extra[@]}" -gt 0 ]; then
        _jobs+=("---" "${_extra[@]}")
    fi
    if [ "${#_jobs[@]}" -eq 0 ]; then
        # As the binary's nothingTo says it.
        case "${_action}" in
            docs) echo "No docs to build." ;;
            bench) echo "Nothing to benchmark." ;;
            *) echo "Nothing to ${_action}." ;;
        esac
        exit 0
    fi
    _runner_flags=()
    [ "${_fail_on_skip}" -eq 0 ] || _runner_flags+=(--fail-on-skip)
    if [ "${_action}" != docs ]; then
        exec "${HECATRON_HOME}/libexec/runner/run-parallel.sh" ${_jflag:+-j "${_jflag}"} ${_sflag:+-s "${_sflag}"} \
            ${_runner_flags[@]+"${_runner_flags[@]}"} "${_action}" "${_jobs[@]}"
    fi
    # hct docs ends with where each language's docs are, from the lines its
    # hooks end with (docs_written in languages/_hooks.sh), as the binary does.
    _status=0
    "${HECATRON_HOME}/libexec/runner/run-parallel.sh" ${_jflag:+-j "${_jflag}"} ${_sflag:+-s "${_sflag}"} \
        ${_runner_flags[@]+"${_runner_flags[@]}"} "${_action}" "${_jobs[@]}" || _status=$?
    _written=()
    for _job in "${_jobs[@]}"; do
        case "${_job}" in
            *::*)
                while IFS= read -r _line; do _written+=("${_line}"); done < <(
                    grep -hE '^[^[:space:]]+: docs: written to ' "${HECATRON_LOGS}/docs/${_job%%::*}.log" 2>/dev/null) ;;
        esac
    done
    if [ "${#_written[@]}" -gt 0 ]; then
        printf '\n'
        printf '%s\n' "${_written[@]}"
    fi
    exit "${_status}"
fi

# --- Named targets, one at a time -----------------------------------------
_run_target() {
    local target="$1" command name
    if command="$(_task_command "${_action}" "${target}")"; then
        bash -c "${command}"
        return
    fi
    if [ "${_action}" = "lint" ] && [ "${target}" = "shell" ]; then
        bash "${HECATRON_HOME}/libexec/lint/shellcheck.sh"
        return
    fi
    name="$(_check "${target}" "${_action}")" || return 1
    if python3 "${HECATRON_HOME}/libexec/lib/project.py" resolve "${name}" "${_action}.sh" >/dev/null; then
        bash "${HECATRON_HOME}/libexec/lib/each-component.sh" "${_action}" "${name}" \
            ${_components[@]+"${_components[@]}"}
        return
    fi
    # Skipped, in the line each-component.sh ends with (the binary's noHook).
    case "${_action}" in
        lint) echo "${name}: lint skipped: ${name} has no separate linter: its build treats warnings as errors (hecatron test ${name})" ;;
        build) echo "${name}: build skipped: no build step for ${name}" ;;
        clean) echo "${name}: clean skipped: no build artifacts for ${name}" ;;
        audit) echo "${name}: audit skipped: nothing to audit for ${name} (hecatron has no auditor for its dependencies)" ;;
        format) echo "${name}: format skipped: no formatter for ${name}" ;;
        deps) echo "${name}: deps skipped: no dependency manager for ${name}" ;;
        outdated) echo "${name}: outdated skipped: no outdated report for ${name}" ;;
        docs) echo "${name}: docs skipped: no docs tool for ${name}" ;;
        bench) echo "${name}: bench skipped: no benchmark runner for ${name}" ;;
        *) echo "error: ${name} has no ${_action} hook (.hecatron/languages/${name}/${_action}.sh)" >&2; return 1 ;;
    esac
}

# Whether a target's run skipped: its log ends with the line saying so.
_skipped_run() {
    tail -n 1 "$1" 2>/dev/null | grep -E "^[^ ]+: ${_action} skipped( for [0-9]+ component\\(s\\))?: " >/dev/null
}

_status=0
_passed=0
mkdir -p "${HECATRON_LOGS}/${_action}"
for _target in "${_targets[@]}"; do
    _log="${HECATRON_LOGS}/${_action}/$(_name "${_target}").log"
    set +e
    ( _run_target "${_target}" ) 2>&1 | tee "${_log}"
    _rc="${PIPESTATUS[0]}"
    set -e
    if [ "${_rc}" -ne 0 ]; then
        _status=1
    elif ! _skipped_run "${_log}"; then
        _passed=$((_passed + 1))
    fi
done
if [ "${_status}" -eq 0 ] && [ "${_passed}" -eq 0 ] && [ "${_fail_on_skip}" -eq 1 ]; then
    echo "hecatron: nothing ran: every job skipped (--fail-on-skip)" >&2
    _status=1
fi
exit "${_status}"

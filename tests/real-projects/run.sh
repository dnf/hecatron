#!/usr/bin/env bash
# Test hecatron on a real open-source project as a newcomer would: clone it,
# then `hct init`, `hct install`, `hct doctor` and `hct test <lang>` for each
# language init found, with no configuration. Each step is timed between
# "### step NAME" and "### done NAME rc=N secs=N" lines, and "### json NAME
# {...}" lines carry hecatron's own JSON (init's config, hct status, each
# test run's --output json); tests/real-projects/report.py reads them.
#
#   run.sh [options] OWNER/REPO          here (a fresh container or CI runner)
#   run.sh --docker IMAGE [options] OWNER/REPO
#                                        in a fresh container of IMAGE, with
#                                        this checkout as hecatron
#                                        (tests/docker/run.sh)
#
# Options:
#   --ref REF          test REF instead of the default branch
#   --actions LIST     comma-separated actions per language (default: test)
#   --budget SECS      stop starting steps after SECS in all (default 3300)
#   --host TEXT        where this runs, for the record (e.g. a runner label)
#   --run-url URL      the CI run, for the record
#
# It installs toolchains, so run it only where that's wanted: a throwaway
# container or a CI runner.
set -uo pipefail

_here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
_image=""
_ref=""
_actions="test"
_budget=3300
_host=""
_run_url=""
_docker=""
while [ "$#" -gt 1 ]; do
    case "$1" in
        --docker) _docker="$2"; shift 2 ;;
        --image) _image="$2"; shift 2 ;;
        --ref) _ref="$2"; shift 2 ;;
        --actions) _actions="$2"; shift 2 ;;
        --budget) _budget="$2"; shift 2 ;;
        --host) _host="$2"; shift 2 ;;
        --run-url) _run_url="$2"; shift 2 ;;
        *) break ;;
    esac
done
if [ "$#" -ne 1 ] || [[ "$1" != */* ]]; then
    sed -n '9,23p' "${BASH_SOURCE[0]}" >&2
    exit 2
fi
_repo="$1"

if [ -n "${_docker}" ]; then
    _inner=(--image "${_docker}" --actions "${_actions}" --budget "${_budget}")
    [ -n "${_ref}" ] && _inner+=(--ref "${_ref}")
    [ -n "${_host}" ] && _inner+=(--host "${_host}")
    [ -n "${_run_url}" ] && _inner+=(--run-url "${_run_url}")
    exec "${_here}/../docker/run.sh" "${_docker}" bash /hecatron/tests/real-projects/run.sh "${_inner[@]}" "${_repo}"
fi

_start=${SECONDS}
_home="$(cd "${_here}/../.." && pwd)"
export PATH="${_home}/bin:${PATH}"

# A JSON string, for the few values this script records itself.
_json_string() {
    python3 -c 'import json, sys; print(json.dumps(sys.argv[1]))' "$1"
}

_remaining() {
    echo $((_budget - (SECONDS - _start)))
}

# Run a step under the time left (at most CAP seconds), between markers.
# Prints nothing and returns 124 when no time is left.
_step() {
    local name="$1" cap="$2"
    shift 2
    local left
    left="$(_remaining)"
    if [ "${left}" -le 30 ]; then
        echo "### skip ${name}: out of time"
        return 124
    fi
    [ "${left}" -lt "${cap}" ] && cap="${left}"
    local begin=${SECONDS}
    echo "### step ${name}: $*"
    timeout --kill-after 30 "${cap}" "$@" </dev/null
    local rc=$?
    echo "### done ${name} rc=${rc} secs=$((SECONDS - begin))"
    return "${rc}"
}

_hecatron_commit="$(git -c safe.directory='*' -C "${_home}" rev-parse HEAD 2>/dev/null || true)"
_hecatron_runner=scripts
[ -x "${_home}/bin/.hecatron-go" ] && [ -z "${HECATRON_NO_GO:-}" ] && _hecatron_runner=binary
_os="$(. /etc/os-release 2>/dev/null && echo "${PRETTY_NAME:-}")"
[ -n "${_os}" ] || _os="$(uname -sr)"
echo "### meta {\"repo\": $(_json_string "${_repo}"), \"date\": \"$(date -u +%Y-%m-%d)\"," \
    "\"hecatron\": {\"version\": \"$(cat "${_home}/VERSION")\", \"commit\": $(_json_string "${_hecatron_commit}"), \"runner\": \"${_hecatron_runner}\"}," \
    "\"runner\": {\"os\": $(_json_string "${_os}"), \"arch\": \"$(uname -m)\", \"image\": $(_json_string "${_image}"), \"host\": $(_json_string "${_host}"), \"runUrl\": $(_json_string "${_run_url}")}," \
    "\"actions\": $(_json_string "${_actions}")}"

_work="${HOME}/$(echo "${_repo}" | tr '/' '_')"
_clone=(git clone --quiet --depth 1 --recurse-submodules --shallow-submodules)
[ -n "${_ref}" ] && _clone+=(--branch "${_ref}")
_end() {
    echo "### end {\"wallSeconds\": $((SECONDS - _start))}"
}
if ! _step clone 900 "${_clone[@]}" "https://github.com/${_repo}.git" "${_work}" || ! cd "${_work}"; then
    _end
    exit 1
fi
echo "### project {\"commit\": \"$(git rev-parse HEAD)\", \"root\": $(_json_string "${PWD}")}"

_step init 300 hct init
if [ -f .hecatron/config.json ]; then
    echo "### json config $(python3 -c 'import json; print(json.dumps(json.load(open(".hecatron/config.json"))))')"
fi
if [ "${_hecatron_runner}" = binary ]; then
    echo "### json status $(hct status --output json 2>/dev/null | python3 -c 'import json, sys; print(json.dumps(json.load(sys.stdin)))' 2>/dev/null || echo null)"
fi

_step install 2400 hct install
_step doctor 300 hct doctor

_languages="$(python3 -c 'import json; print(" ".join(json.load(open(".hecatron/config.json"))["languages"]))' 2>/dev/null)"
_logs="$(python3 -c 'import json; print(json.load(open(".hecatron/config.json")).get("logs") or ".logs")' 2>/dev/null || echo .logs)"
_out="$(mktemp)"
for _lang in ${_languages}; do
    for _action in ${_actions//,/ }; do
        if [ "${_hecatron_runner}" = binary ]; then
            # The JSON goes to a file; the tool's output is in the job's log,
            # printed inside the step so its test counts can be read.
            _step "${_action}:${_lang}" 1800 bash -c 'hct "$1" "$2" --output json > "$3"; rc=$?; cat "$4/$1/$2.log" 2>/dev/null; exit "${rc}"' \
                _ "${_action}" "${_lang}" "${_out}" "${_logs}"
            echo "### json ${_action}:${_lang} $(python3 -c 'import json, sys; print(json.dumps(json.load(open(sys.argv[1]))))' "${_out}" 2>/dev/null || echo null)"
        else
            _step "${_action}:${_lang}" 1800 hct "${_action}" "${_lang}"
        fi
    done
done
rm -f "${_out}"
_end

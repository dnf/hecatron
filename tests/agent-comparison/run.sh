#!/usr/bin/env bash
# The agent comparison (docs/contributing/agent-comparison.md): Claude Code, given
# the same prompt, gets a repository's tests running on a fresh machine,
# with hecatron and its Agent Skill installed or without. Each run is a new
# container; its result is Claude Code's own stream (tokens, cost, turns,
# time) and whether the test command the agent recorded passes when the
# harness runs it again afterwards.
#
#   run.sh RESULTS [RUNS] [PARALLEL]   every repository in repos.json, each arm, RUNS times
#
# ENV_FILE names a docker --env-file with the agent's credentials
# (CLAUDE_CODE_OAUTH_TOKEN=... or ANTHROPIC_API_KEY=...), read only by
# docker. MODEL picks the model (default claude-sonnet-5-5); ONLY runs one
# repository by name.

set -uo pipefail

_here="$(cd "$(dirname "$0")" && pwd)"
_results="${1:?usage: run.sh RESULTS [RUNS] [PARALLEL]}"
_runs="${2:-3}"
_parallel="${3:-3}"
_model="${MODEL:-claude-sonnet-5-5}"
_env_file="${ENV_FILE:?ENV_FILE: a docker --env-file with CLAUDE_CODE_OAUTH_TOKEN or ANTHROPIC_API_KEY}"
_timeout="${TIMEOUT:-45m}"

# What the agent is asked, the same in both arms ({scope} narrows a
# repository with several implementations).
_prompt='You are on a fresh Ubuntu 24.04 machine with passwordless sudo and network access, in a clone of {url}. Get the repository'"'"'s test suite running{scope}, installing whatever it needs. Do not change the repository'"'"'s code or tests to make them pass. When you are done, write the shell command that runs those tests from the repository'"'"'s root to ~/TEST_COMMAND (one line; it will be run in a new login shell), and say how many tests passed and failed.'

latest_release() {
    curl -fsSI https://github.com/dnf/hecatron/releases/latest | sed -n 's|^location: .*/tag/\(v[^[:space:]]*\).*|\1|ip'
}

images() {
    docker build -q --target plain -t hecatron-agent-comparison:plain "${_here}" >/dev/null &&
        docker build -q --target hecatron --build-arg HECATRON_RELEASE="$(latest_release)" \
            -t hecatron-agent-comparison:hecatron "${_here}" >/dev/null
}

# One run: repository index, arm, run number. Writes RESULTS/<repo>/<arm>-<n>/.
one() {
    local _index="$1" _arm="$2" _n="$3" _name _url _commit _scope _out _prompt_text
    _name="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))[int(sys.argv[2])]["name"])' "${_here}/repos.json" "${_index}")"
    _url="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))[int(sys.argv[2])]["url"])' "${_here}/repos.json" "${_index}")"
    _commit="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))[int(sys.argv[2])]["commit"])' "${_here}/repos.json" "${_index}")"
    _scope="$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))[int(sys.argv[2])]["scope"])' "${_here}/repos.json" "${_index}")"
    _out="${_results}/${_name}/${_arm}-${_n}"
    if [ -f "${_out}/verify.txt" ]; then
        echo "skip ${_name} ${_arm} ${_n} (done)"
        return 0
    fi
    mkdir -p "${_out}"
    _prompt_text="${_prompt//\{url\}/${_url}}"
    _prompt_text="${_prompt_text//\{scope\}/${_scope}}"
    printf '%s\n' "${_prompt_text}" >"${_out}/prompt.txt"
    echo "start ${_name} ${_arm} ${_n}"
    # The agent's work, then the harness's check of the command it recorded.
    docker run --rm --env-file "${_env_file}" -v "${_out}:/out" \
        -e PROMPT="${_prompt_text}" -e MODEL="${_model}" -e URL="${_url}" -e COMMIT="${_commit}" -e TIMEOUT="${_timeout}" \
        "hecatron-agent-comparison:${_arm}" bash -lc '
            git clone -q "${URL}" repo && git -C repo checkout -q "${COMMIT}" || exit 1
            cd repo
            start=$(date +%s)
            timeout "${TIMEOUT}" claude -p "${PROMPT}" --model "${MODEL}" --dangerously-skip-permissions \
                --output-format stream-json --verbose >/out/stream.jsonl 2>/out/stderr.txt
            echo "agent_exit=$? seconds=$(( $(date +%s) - start ))" >/out/agent.txt
            if [ -s ~/TEST_COMMAND ]; then
                cp ~/TEST_COMMAND /out/test_command.txt
                bash -lc "cd ~/repo && $(head -n 1 ~/TEST_COMMAND)" >/out/verify.log 2>&1
                echo "exit=$?" >/out/verify.txt
            else
                echo "exit=none" >/out/verify.txt
            fi' >"${_out}/docker.log" 2>&1
    echo "done ${_name} ${_arm} ${_n}: $(cat "${_out}/verify.txt" 2>/dev/null || echo 'no result')"
}

main() {
    mkdir -p "${_results}"
    images || { echo "error: the images didn't build" >&2; exit 1; }
    local _count _index _arm _n
    _count="$(python3 -c 'import json,sys; print(len(json.load(open(sys.argv[1]))))' "${_here}/repos.json")"
    for _n in $(seq 1 "${_runs}"); do
        for _index in $(seq 0 $((_count - 1))); do
            if [ -n "${ONLY:-}" ] && [ "$(python3 -c 'import json,sys; print(json.load(open(sys.argv[1]))[int(sys.argv[2])]["name"])' "${_here}/repos.json" "${_index}")" != "${ONLY}" ]; then
                continue
            fi
            for _arm in plain hecatron; do
                printf '%s %s %s\n' "${_index}" "${_arm}" "${_n}"
            done
        done
    done | xargs -P "${_parallel}" -L 1 bash "$0" __one "${_results}"
}

if [ "${1:-}" = __one ]; then
    _results="$2"
    shift 2
    one "$@"
else
    main
fi

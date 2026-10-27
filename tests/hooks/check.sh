#!/usr/bin/env bash
# Check a language's default hooks on this machine: install the language,
# copy a fixture project (tests/hooks/fixtures/<lang>/<fixture>/) into a new
# project as that language's directory, and run hecatron's test, lint, build
# and clean on it. Each fixture is a small library in one build system with
# a real test, so a hook that runs nothing fails.
#
# Runs where hecatron is on PATH, normally a throwaway container:
#   tests/docker/run.sh ubuntu:24.04 bash /hecatron/tests/hooks/check.sh go
#   tests/hooks/check.sh java maven    # one fixture
set -uo pipefail

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
    echo "usage: $0 <lang> [fixture]" >&2
    exit 2
fi
_lang="$1"
_home="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
_fixtures="${_home}/tests/hooks/fixtures/${_lang}"
if [ "$#" -eq 2 ]; then _names=("$2"); else mapfile -t _names < <(ls "${_fixtures}"); fi

_failed=0
for _fixture in "${_names[@]}"; do
    _project="$(mktemp -d)"
    cp -R "${_fixtures}/${_fixture}" "${_project}/${_lang}"
    (cd "${_project}" && git init -q && git add -A && hecatron init >/dev/null && hecatron add "${_lang}" >/dev/null)
    echo "### ${_lang}/${_fixture}: install"
    (cd "${_project}" && hecatron install "${_lang}" && hecatron doctor "${_lang}") || _failed=1
    _results=""
    for _action in test lint build clean; do
        echo "### ${_lang}/${_fixture}: ${_action}"
        if (cd "${_project}" && hecatron "${_action}" "${_lang}"); then
            _results="${_results} ${_action}=pass"
        else
            _results="${_results} ${_action}=fail"
            _failed=1
        fi
    done
    echo "### hooks ${_lang}/${_fixture}:${_results}"
done
exit "${_failed}"

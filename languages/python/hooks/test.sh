#!/usr/bin/env bash
# Default test hook: pytest, in the project's environment (with every extra,
# where test dependencies usually are) through uv, else on its own. Without
# a project here, the projects in the directories below, each (a monorepo's
# py-polars/ or python-package/), else pytest here on its own. Asked for a
# JUnit report (HECATRON_TEST_REPORTS), pytest writes one there.
source "$(dirname "$0")/../../_hooks.sh"
has_any pyproject.toml setup.py setup.cfg || run_below_with pyproject.toml setup.py setup.cfg
_junit=()
if [ -n "${HECATRON_TEST_REPORTS:-}" ]; then _junit=(--junitxml="${HECATRON_TEST_REPORTS}/pytest.xml"); fi
if [ -f pyproject.toml ] || [ -f setup.py ] || [ -f setup.cfg ]; then
    run uv run --all-extras --with pytest pytest ${_junit[@]+"${_junit[@]}"}
else
    run uv run --with pytest pytest ${_junit[@]+"${_junit[@]}"}
fi
